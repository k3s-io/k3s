package etcdsnapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cluster"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/server"
	util2 "github.com/k3s-io/k3s/pkg/util"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

// commandSetup setups up common things needed
// for each etcd command.
func commandSetup(app *cli.Context, cfg *cmds.Server, sc *server.Config) error {
	gspt.SetProcTitle(os.Args[0])

	nodeName := app.String("node-name")
	if nodeName == "" {
		h, err := os.Hostname()
		if err != nil {
			return err
		}
		nodeName = h
	}

	os.Setenv("NODE_NAME", nodeName)

	dataDir, err := server.ResolveDataDir(cfg.DataDir)
	if err != nil {
		return err
	}

	sc.DisableAgent = true
	sc.ControlConfig.DataDir = dataDir
	sc.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
	sc.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
	sc.ControlConfig.EtcdSnapshotCompress = cfg.EtcdSnapshotCompress
	sc.ControlConfig.EtcdListFormat = strings.ToLower(cfg.EtcdListFormat)
	sc.ControlConfig.EtcdS3 = cfg.EtcdS3
	sc.ControlConfig.EtcdS3Endpoint = cfg.EtcdS3Endpoint
	sc.ControlConfig.EtcdS3EndpointCA = cfg.EtcdS3EndpointCA
	sc.ControlConfig.EtcdS3SkipSSLVerify = cfg.EtcdS3SkipSSLVerify
	sc.ControlConfig.EtcdS3AccessKey = cfg.EtcdS3AccessKey
	sc.ControlConfig.EtcdS3SecretKey = cfg.EtcdS3SecretKey
	sc.ControlConfig.EtcdS3BucketName = cfg.EtcdS3BucketName
	sc.ControlConfig.EtcdS3Region = cfg.EtcdS3Region
	sc.ControlConfig.EtcdS3Folder = cfg.EtcdS3Folder
	sc.ControlConfig.EtcdS3Insecure = cfg.EtcdS3Insecure
	sc.ControlConfig.EtcdS3Timeout = cfg.EtcdS3Timeout
	sc.ControlConfig.Runtime = &config.ControlRuntime{}
	sc.ControlConfig.Runtime.ETCDServerCA = filepath.Join(dataDir, "tls", "etcd", "server-ca.crt")
	sc.ControlConfig.Runtime.ClientETCDCert = filepath.Join(dataDir, "tls", "etcd", "client.crt")
	sc.ControlConfig.Runtime.ClientETCDKey = filepath.Join(dataDir, "tls", "etcd", "client.key")
	sc.ControlConfig.Runtime.KubeConfigAdmin = filepath.Join(dataDir, "cred", "admin.kubeconfig")

	return nil
}

// Run is an alias for Save, retained for compatibility reasons.
func Run(app *cli.Context) error {
	return Save(app)
}

// Save triggers an on-demand etcd snapshot operation
func Save(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return save(app, &cmds.ServerConfig)
}

func save(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	if err := commandSetup(app, cfg, &serverConfig); err != nil {
		return err
	}

	if len(app.Args()) > 0 {
		return util2.ErrCommandNoArgs
	}

	serverConfig.ControlConfig.EtcdSnapshotRetention = 0 // disable retention check

	ctx := signals.SetupSignalContext()
	e := etcd.NewETCD()
	if err := e.SetControlConfig(ctx, &serverConfig.ControlConfig); err != nil {
		return err
	}

	initialized, err := e.IsInitialized(ctx, &serverConfig.ControlConfig)
	if err != nil {
		return err
	}
	if !initialized {
		return fmt.Errorf("etcd database not found in %s", serverConfig.ControlConfig.DataDir)
	}

	cluster := cluster.New(&serverConfig.ControlConfig)

	if err := cluster.Bootstrap(ctx, true); err != nil {
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

	if err := commandSetup(app, cfg, &serverConfig); err != nil {
		return err
	}

	snapshots := app.Args()
	if len(snapshots) == 0 {
		return errors.New("no snapshots given for removal")
	}

	ctx := signals.SetupSignalContext()
	e := etcd.NewETCD()
	if err := e.SetControlConfig(ctx, &serverConfig.ControlConfig); err != nil {
		return err
	}

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

var etcdListFormats = []string{"json", "yaml"}

func validEtcdListFormat(format string) bool {
	for _, supportedFormat := range etcdListFormats {
		if format == supportedFormat {
			return true
		}
	}
	return false
}

func list(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	if err := commandSetup(app, cfg, &serverConfig); err != nil {
		return err
	}

	ctx := signals.SetupSignalContext()
	e := etcd.NewETCD()
	if err := e.SetControlConfig(ctx, &serverConfig.ControlConfig); err != nil {
		return err
	}

	sf, err := e.ListSnapshots(ctx)
	if err != nil {
		return err
	}

	if cfg.EtcdListFormat != "" && !validEtcdListFormat(cfg.EtcdListFormat) {
		return errors.New("invalid output format: " + cfg.EtcdListFormat)
	}

	switch cfg.EtcdListFormat {
	case "json":
		if err := json.NewEncoder(os.Stdout).Encode(sf); err != nil {
			return err
		}
		return nil
	case "yaml":
		if err := yaml.NewEncoder(os.Stdout).Encode(sf); err != nil {
			return err
		}
		return nil
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
		defer w.Flush()

		if cfg.EtcdS3 {
			fmt.Fprint(w, "Name\tSize\tCreated\n")
			for _, s := range sf {
				if s.NodeName == "s3" {
					fmt.Fprintf(w, "%s\t%d\t%s\n", s.Name, s.Size, s.CreatedAt.Format(time.RFC3339))
				}
			}
		} else {
			fmt.Fprint(w, "Name\tLocation\tSize\tCreated\n")
			for _, s := range sf {
				if s.NodeName != "s3" {
					fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", s.Name, s.Location, s.Size, s.CreatedAt.Format(time.RFC3339))
				}
			}
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

	if err := commandSetup(app, cfg, &serverConfig); err != nil {
		return err
	}

	serverConfig.ControlConfig.EtcdSnapshotRetention = cfg.EtcdSnapshotRetention

	ctx := signals.SetupSignalContext()
	e := etcd.NewETCD()
	if err := e.SetControlConfig(ctx, &serverConfig.ControlConfig); err != nil {
		return err
	}

	sc, err := server.NewContext(ctx, serverConfig.ControlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.Runtime.Core = sc.Core

	return e.PruneSnapshots(ctx)
}
