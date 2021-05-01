package etcdsnapshot

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cluster"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/wrangler/pkg/signals"
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

	nodeName := app.String("node-name")
	if nodeName == "" {
		h, err := os.Hostname()
		if err != nil {
			return err
		}
		nodeName = h
	}

	os.Setenv("NODE_NAME", nodeName)

	var serverConfig server.Config
	serverConfig.DisableAgent = true
	serverConfig.ControlConfig.DataDir = dataDir
	serverConfig.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
	serverConfig.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
	serverConfig.ControlConfig.EtcdSnapshotRetention = 0 // disable retention check
	serverConfig.ControlConfig.EtcdS3 = cfg.EtcdS3
	serverConfig.ControlConfig.EtcdS3Endpoint = cfg.EtcdS3Endpoint
	serverConfig.ControlConfig.EtcdS3EndpointCA = cfg.EtcdS3EndpointCA
	serverConfig.ControlConfig.EtcdS3SkipSSLVerify = cfg.EtcdS3SkipSSLVerify
	serverConfig.ControlConfig.EtcdS3AccessKey = cfg.EtcdS3AccessKey
	serverConfig.ControlConfig.EtcdS3SecretKey = cfg.EtcdS3SecretKey
	serverConfig.ControlConfig.EtcdS3BucketName = cfg.EtcdS3BucketName
	serverConfig.ControlConfig.EtcdS3Region = cfg.EtcdS3Region
	serverConfig.ControlConfig.EtcdS3Folder = cfg.EtcdS3Folder
	serverConfig.ControlConfig.Runtime = &config.ControlRuntime{}
	serverConfig.ControlConfig.Runtime.ETCDServerCA = filepath.Join(dataDir, "tls", "etcd", "server-ca.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDCert = filepath.Join(dataDir, "tls", "etcd", "client.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDKey = filepath.Join(dataDir, "tls", "etcd", "client.key")
	serverConfig.ControlConfig.Runtime.KubeConfigAdmin = filepath.Join(dataDir, "cred", "admin.kubeconfig")

	ctx := signals.SetupSignalHandler(context.Background())

	initialized, err := etcd.NewETCD().IsInitialized(ctx, &serverConfig.ControlConfig)
	if err != nil {
		return err
	}
	if !initialized {
		return errors.New("managed etcd database has not been initialized")
	}

	cluster := cluster.New(&serverConfig.ControlConfig)

	if err := cluster.Bootstrap(ctx); err != nil {
		return err
	}

	sc, err := server.NewContext(ctx, serverConfig.ControlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.Runtime.Core = sc.Core

	return cluster.Snapshot(ctx, &serverConfig.ControlConfig)
}
