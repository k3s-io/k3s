package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
)

// Embedded is defined here so that we can use embedded.ETCD even when the rest
// of the embedded execututor is disabled by build flags
type Embedded struct {
	apiServerReady <-chan struct{}
	etcdReady      chan struct{}
	criReady       chan struct{}
	nodeConfig     *daemonconfig.Node
}

func (e *Embedded) ETCD(ctx context.Context, wg *sync.WaitGroup, args *ETCDConfig, extraArgs []string, test TestFunc) error {
	// An unbootstrapped executor is used to start up a temporary embedded etcd when reconciling.
	// This temporary executor doesn't have any ready channels set up, so don't bother testing.
	if e.etcdReady != nil {
		go func() {
			for {
				if err := test(ctx, true); err != nil {
					logrus.Infof("Failed to test etcd connection: %v", err)
				} else {
					logrus.Info("Connection to etcd is ready")
					close(e.etcdReady)
					return
				}

				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// nil args indicates a no-op start; all we need to do is wait for the test
	// func to indicate readiness and close the channel.
	if args == nil {
		return nil
	}

	configFile, err := args.ToConfigFile(extraArgs)
	if err != nil {
		return err
	}
	cfg, err := embed.ConfigFromFile(configFile)
	if err != nil {
		return err
	}

	wg.Add(1)
	etcd, err := embed.StartEtcd(cfg)
	if err != nil {
		wg.Done()
		return err
	}

	go func() {
		<-ctx.Done()
		logrus.Infof("Stopping etcd server...")
		etcd.Server.Stop()
		go etcd.Close()
	}()

	go func() {
		defer wg.Done()
		for {
			select {
			case err := <-etcd.Server.ErrNotify():
				if errors.Is(err, rafthttp.ErrMemberRemoved) {
					tombstoneFile := filepath.Join(args.DataDir, "tombstone")
					if err := os.WriteFile(tombstoneFile, []byte{}, 0600); err != nil {
						logrus.Fatalf("Failed to write tombstone file to %s: %v", tombstoneFile, err)
					}
					etcd.Close()
					logrus.Infof("This node has been removed from the cluster - please restart %s to rejoin the cluster", version.Program)
					return
				}
				logrus.Errorf("etcd error: %v", err)
			case <-etcd.Server.StopNotify():
				logrus.Info("etcd server stopped")
				return
			case err := <-etcd.Err():
				logrus.Errorf("etcd exited: %v", err)
				return
			}
		}
	}()
	return nil
}
