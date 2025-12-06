package etcd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
)

// StartETCD runs an embedded etcd server instance with the provided config.
// This function will return if the server has been successfully started.
// The server will continue to run until the context is cancelled or some internal error occurs.
func StartETCD(ctx context.Context, wg *sync.WaitGroup, args *executor.ETCDConfig, extraArgs []string) error {
	// nil args indicates a no-op start
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
