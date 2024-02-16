package etcdsnapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/util"
	util2 "github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
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
	config.ControlConfig.BindAddress = cfg.BindAddress
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

	// We need to go through defaulting of cluster addresses to ensure that the etcd config for the standalone
	// command uses the same endpoint selection logic as it does when starting up the full server. Specifically,
	// we need to set an IPv6 service CIDR on IPv6-only or IPv6-first nodes, as the etcd default endpoints check
	// the service CIDR primary addresss family to determine what loopback address to use.
	nodeName, nodeIPs, err := util.GetHostnameAndIPs(cmds.AgentConfig.NodeName, cmds.AgentConfig.NodeIP)
	if err != nil {
		return nil, err
	}
	config.ControlConfig.ServerNodeName = nodeName

	// configure ClusterIPRanges. Use default 10.42.0.0/16 or fd00:42::/56 if user did not set it
	_, defaultClusterCIDR, defaultServiceCIDR, _ := util.GetDefaultAddresses(nodeIPs[0])
	if len(cfg.ClusterCIDR) == 0 {
		cfg.ClusterCIDR.Set(defaultClusterCIDR)
	}
	for _, cidr := range util.SplitStringSlice(cfg.ClusterCIDR) {
		_, parsed, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid cluster-cidr %s", cidr)
		}
		config.ControlConfig.ClusterIPRanges = append(config.ControlConfig.ClusterIPRanges, parsed)
	}

	// set ClusterIPRange to the first address (first defined IPFamily is preferred)
	config.ControlConfig.ClusterIPRange = config.ControlConfig.ClusterIPRanges[0]

	// configure ServiceIPRanges. Use default 10.43.0.0/16 or fd00:43::/112 if user did not set it
	if len(cfg.ServiceCIDR) == 0 {
		cfg.ServiceCIDR.Set(defaultServiceCIDR)
	}
	for _, cidr := range util.SplitStringSlice(cfg.ServiceCIDR) {
		_, parsed, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid service-cidr %s", cidr)
		}
		config.ControlConfig.ServiceIPRanges = append(config.ControlConfig.ServiceIPRanges, parsed)
	}

	// set ServiceIPRange to the first address (first defined IPFamily is preferred)
	config.ControlConfig.ServiceIPRange = config.ControlConfig.ServiceIPRanges[0]

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

	sc, err := server.NewContext(ctx, config, false)
	if err != nil {
		return nil, err
	}
	config.ControlConfig.Runtime.K3s = sc.K3s
	config.ControlConfig.Runtime.Core = sc.Core

	return &etcdCommand{etcd: e, ctx: ctx}, nil
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

		// Sort snapshots by creation time and key
		sfKeys := make([]string, 0, len(sf))
		for k := range sf {
			sfKeys = append(sfKeys, k)
		}
		sort.Slice(sfKeys, func(i, j int) bool {
			iKey := sfKeys[i]
			jKey := sfKeys[j]
			if sf[iKey].CreatedAt.Equal(sf[jKey].CreatedAt) {
				return iKey < jKey
			}
			return sf[iKey].CreatedAt.Before(sf[jKey].CreatedAt)
		})

		fmt.Fprint(w, "Name\tLocation\tSize\tCreated\n")
		for _, k := range sfKeys {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", sf[k].Name, sf[k].Location, sf[k].Size, sf[k].CreatedAt.Format(time.RFC3339))
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
