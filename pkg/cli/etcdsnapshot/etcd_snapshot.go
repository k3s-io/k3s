package etcdsnapshot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cluster"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/rancher/k3s/pkg/server"
	util2 "github.com/rancher/k3s/pkg/util"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/urfave/cli"
)

// commandSetup setups up common things needed
// for each etcd command.
func commandSetup(app *cli.Context, cfg *cmds.Server, sc *server.Config) (string, error) {
	gspt.SetProcTitle(os.Args[0])

	nodeName := app.String("node-name")
	if nodeName == "" {
		h, err := os.Hostname()
		if err != nil {
			return "", err
		}
		nodeName = h
	}

	os.Setenv("NODE_NAME", nodeName)

	sc.DisableAgent = true
	sc.ControlConfig.DataDir = cfg.DataDir
	sc.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
	sc.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
	sc.ControlConfig.EtcdS3 = cfg.EtcdS3
	sc.ControlConfig.EtcdS3Endpoint = cfg.EtcdS3Endpoint
	sc.ControlConfig.EtcdS3EndpointCA = cfg.EtcdS3EndpointCA
	sc.ControlConfig.EtcdS3SkipSSLVerify = cfg.EtcdS3SkipSSLVerify
	sc.ControlConfig.EtcdS3AccessKey = cfg.EtcdS3AccessKey
	sc.ControlConfig.EtcdS3SecretKey = cfg.EtcdS3SecretKey
	sc.ControlConfig.EtcdS3BucketName = cfg.EtcdS3BucketName
	sc.ControlConfig.EtcdS3Region = cfg.EtcdS3Region
	sc.ControlConfig.EtcdS3Folder = cfg.EtcdS3Folder
	sc.ControlConfig.Runtime = &config.ControlRuntime{}

	return server.ResolveDataDir(cfg.DataDir)
}

func Run(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return run(app, &cmds.ServerConfig)
}

func run(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	dataDir, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	if len(app.Args()) > 0 {
		return util2.ErrCommandNoArgs
	}

	serverConfig.ControlConfig.DataDir = dataDir
	serverConfig.ControlConfig.EtcdSnapshotRetention = 0 // disable retention check
	serverConfig.ControlConfig.Runtime.ETCDServerCA = filepath.Join(dataDir, "tls", "etcd", "server-ca.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDCert = filepath.Join(dataDir, "tls", "etcd", "client.crt")
	serverConfig.ControlConfig.Runtime.ClientETCDKey = filepath.Join(dataDir, "tls", "etcd", "client.key")
	serverConfig.ControlConfig.Runtime.KubeConfigAdmin = filepath.Join(dataDir, "cred", "admin.kubeconfig")

	ctx := signals.SetupSignalHandler(context.Background())
	e := etcd.NewETCD()
	e.SetControlConfig(&serverConfig.ControlConfig)

	initialized, err := e.IsInitialized(ctx, &serverConfig.ControlConfig)
	if err != nil {
		return err
	}
	if !initialized {
		return fmt.Errorf("etcd database not found in %s", dataDir)
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

func Delete(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return delete(app, &cmds.ServerConfig)
}

func delete(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	dataDir, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	snapshots := app.Args()
	if len(snapshots) == 0 {
		return errors.New("no snapshots given for removal")
	}

	serverConfig.ControlConfig.DataDir = dataDir
	serverConfig.ControlConfig.Runtime.KubeConfigAdmin = filepath.Join(dataDir, "cred", "admin.kubeconfig")

	ctx := signals.SetupSignalHandler(context.Background())
	e := etcd.NewETCD()
	e.SetControlConfig(&serverConfig.ControlConfig)

	sc, err := server.NewContext(ctx, serverConfig.ControlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.Runtime.Core = sc.Core

	return e.DeleteSnapshots(ctx, app.Args())
}

func List(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return list(app, &cmds.ServerConfig)
}

func list(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	dataDir, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	serverConfig.ControlConfig.DataDir = dataDir

	ctx := signals.SetupSignalHandler(context.Background())
	e := etcd.NewETCD()
	e.SetControlConfig(&serverConfig.ControlConfig)

	sf, err := e.ListSnapshots(ctx)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	defer w.Flush()

	for _, s := range sf {
		if cfg.EtcdS3 {
			fmt.Fprintf(w, "%s\t%d\t%s\n", s.Name, s.Size, s.CreatedAt.Format(time.RFC3339))
		} else {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", s.Name, s.Location, s.Size, s.CreatedAt.Format(time.RFC3339))
		}
	}

	return nil
}

func Prune(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return prune(app, &cmds.ServerConfig)
}

func prune(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	dataDir, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	serverConfig.ControlConfig.DataDir = dataDir
	serverConfig.ControlConfig.EtcdSnapshotRetention = cfg.EtcdSnapshotRetention

	ctx := signals.SetupSignalHandler(context.Background())
	e := etcd.NewETCD()
	e.SetControlConfig(&serverConfig.ControlConfig)

	return e.PruneSnapshots(ctx)
}
