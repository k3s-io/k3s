package etcdsnapshot

import (
	"context"
	"os"
	"path/filepath"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cluster"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func Run(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return run(app, &cmds.ServerConfig)
}

func run(app *cli.Context, cfg *cmds.Server) error {
	gspt.SetProcTitle(os.Args[0])

	dataDir, err := server.ResolveDataDir(cfg.DataDir)
	if err != nil {
		return err
	}

	var serverConfig server.Config
	serverConfig.DisableAgent = true
	serverConfig.ControlConfig.DataDir = dataDir
	serverConfig.ControlConfig.Datastore.Endpoint = "etcd"
	serverConfig.ControlConfig.EtcdSnapshotNow = true
	serverConfig.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
	serverConfig.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
	serverConfig.ControlConfig.EtcdSnapshotRetention = cfg.EtcdSnapshotRetention
	serverConfig.ControlConfig.Runtime = &config.ControlRuntime{}
	serverConfig.ControlConfig.Runtime.ETCDServerCA = filepath.Join(dataDir, "tls", "etcd", "server-ca.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDCert = filepath.Join(dataDir, "tls", "etcd", "client.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDKey = filepath.Join(dataDir, "tls", "etcd", "client.key")

	ctx := signals.SetupSignalHandler(context.Background())

	cluster := cluster.New(&serverConfig.ControlConfig)

	if err := cluster.Bootstrap(ctx); err != nil {
		return err
	}

	if err := cluster.Snapshot(ctx, &serverConfig.ControlConfig); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	return nil
}
