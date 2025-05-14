package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func (e *Embedded) ETCD(ctx context.Context, args *ETCDConfig, extraArgs []string, test TestFunc) error {
	// An unbootstrapped executor is used to start up a temporary embedded etcd when reconciling.
	// This temporary executor doesn't have any ready channels set up, so don't bother testing.
	if e.etcdReady != nil {
		go func() {
			defer close(e.etcdReady)
			for {
				if err := test(ctx); err != nil {
					logrus.Infof("Failed to test etcd connection: %v", err)
				} else {
					logrus.Info("Connection to etcd is ready")
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

	etcd, err := embed.StartEtcd(cfg)
	if err != nil {
		return err
	}

	go func() {
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
		case <-ctx.Done():
			logrus.Infof("stopping etcd")
			etcd.Close()
		case <-etcd.Server.StopNotify():
			logrus.Fatalf("etcd stopped")
		case err := <-etcd.Err():
			logrus.Fatalf("etcd exited: %v", err)
		}
	}()
	return nil
}
