package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/rafthttp"
)

type Embedded struct {
	nodeConfig *daemonconfig.Node
}

func (e *Embedded) ETCD(ctx context.Context, args ETCDConfig, extraArgs []string) error {
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
