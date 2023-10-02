package etcdsnapshot

import (
	"context"
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
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/server"
	util2 "github.com/k3s-io/k3s/pkg/util"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

type etcdCommand struct {
	etcd *etcd.ETCD
	ctx  context.Context
}

// commandSetup setups up common things needed
// for each etcd command.
func commandSetup(app *cli.Context, cfg *cmds.Server, config *server.Config) (*etcdCommand, error) {
	ctx := signals.SetupSignalContext()
	gspt.SetProcTitle(os.Args[0])

	nodeName := app.String("node-name")
	if nodeName == "" {
		h, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		nodeName = h
	}

	os.Setenv("NODE_NAME", nodeName)

	dataDir, err := server.ResolveDataDir(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	config.DisableAgent = true
	config.ControlConfig.DataDir = dataDir
	config.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
	config.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
	config.ControlConfig.EtcdSnapshotCompress = cfg.EtcdSnapshotCompress
	config.ControlConfig.EtcdListFormat = strings.ToLower(cfg.EtcdListFormat)
	config.ControlConfig.EtcdS3 = cfg.EtcdS3
	config.ControlConfig.EtcdS3Endpoint = cfg.EtcdS3Endpoint
	config.ControlConfig.EtcdS3EndpointCA = cfg.EtcdS3EndpointCA
	config.ControlConfig.EtcdS3SkipSSLVerify = cfg.EtcdS3SkipSSLVerify
	config.ControlConfig.EtcdS3AccessKey = cfg.EtcdS3AccessKey
	config.ControlConfig.EtcdS3SecretKey = cfg.EtcdS3SecretKey
	config.ControlConfig.EtcdS3BucketName = cfg.EtcdS3BucketName
	config.ControlConfig.EtcdS3Region = cfg.EtcdS3Region
	config.ControlConfig.EtcdS3Folder = cfg.EtcdS3Folder
	config.ControlConfig.EtcdS3Insecure = cfg.EtcdS3Insecure
	config.ControlConfig.EtcdS3Timeout = cfg.EtcdS3Timeout
	config.ControlConfig.Runtime = daemonconfig.NewRuntime(nil)
	config.ControlConfig.Runtime.ETCDServerCA = filepath.Join(dataDir, "tls", "etcd", "server-ca.crt")
	config.ControlConfig.Runtime.ClientETCDCert = filepath.Join(dataDir, "tls", "etcd", "client.crt")
	config.ControlConfig.Runtime.ClientETCDKey = filepath.Join(dataDir, "tls", "etcd", "client.key")
	config.ControlConfig.Runtime.KubeConfigAdmin = filepath.Join(dataDir, "cred", "admin.kubeconfig")

	e := etcd.NewETCD()
	if err := e.SetControlConfig(&config.ControlConfig); err != nil {
		return nil, err
	}

	initialized, err := e.IsInitialized()
	if err != nil {
		return nil, err
	}
	if !initialized {
		return nil, fmt.Errorf("etcd database not found in %s", config.ControlConfig.DataDir)
	}

	sc, err := server.NewContext(ctx, config.ControlConfig.Runtime.KubeConfigAdmin, false)
	if err != nil {
		return nil, err
	}
	config.ControlConfig.Runtime.K3s = sc.K3s
	config.ControlConfig.Runtime.Core = sc.Core

	return &etcdCommand{etcd: e, ctx: ctx}, nil
}

// Run was an alias for Save
func Run(app *cli.Context) error {
	cli.ShowAppHelp(app)
	return fmt.Errorf("saving with etcd-snapshot was deprecated in v1.26, use \"etcd-snapshot save\" instead")
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

	if len(app.Args()) > 0 {
		return util2.ErrCommandNoArgs
	}

	ec, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	serverConfig.ControlConfig.EtcdSnapshotRetention = 0 // disable retention check

	return ec.etcd.Snapshot(ec.ctx)
}

func Delete(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return delete(app, &cmds.ServerConfig)
}

func delete(app *cli.Context, cfg *cmds.Server) error {
	var serverConfig server.Config

	ec, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	snapshots := app.Args()
	if len(snapshots) == 0 {
		return errors.New("no snapshots given for removal")
	}

	return ec.etcd.DeleteSnapshots(ec.ctx, app.Args())
}

func List(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return list(app, &cmds.ServerConfig)
}

var etcdListFormats = []string{"json", "yaml", "table"}

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

	ec, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	sf, err := ec.etcd.ListSnapshots(ec.ctx)
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

		fmt.Fprint(w, "Name\tLocation\tSize\tCreated\n")
		for _, s := range sf {
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

	ec, err := commandSetup(app, cfg, &serverConfig)
	if err != nil {
		return err
	}

	serverConfig.ControlConfig.EtcdSnapshotRetention = cfg.EtcdSnapshotRetention

	return ec.etcd.PruneSnapshots(ec.ctx)
}
