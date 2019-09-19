package server

import (
	"context"
	"fmt"
	net2 "net"
	"os"
	"path/filepath"
	"strings"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/netutil"
	"github.com/rancher/k3s/pkg/rootless"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/util/net"
	"k8s.io/kubernetes/pkg/master"

	_ "github.com/go-sql-driver/mysql" // ensure we have mysql
	_ "github.com/lib/pq"              // ensure we have postgres
	_ "github.com/mattn/go-sqlite3"    // ensure we have sqlite
)

func Run(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return run(app, &cmds.ServerConfig)
}

func run(app *cli.Context, cfg *cmds.Server) error {
	var (
		err error
	)

	if !cfg.DisableAgent && os.Getuid() != 0 && !cfg.Rootless {
		return fmt.Errorf("must run as root unless --disable-agent is specified")
	}

	if cfg.Rootless {
		dataDir, err := datadir.LocalHome(cfg.DataDir, true)
		if err != nil {
			return err
		}
		cfg.DataDir = dataDir
		if err := rootless.Rootless(dataDir); err != nil {
			return err
		}
	}

	serverConfig := server.Config{}
	serverConfig.ControlConfig.ClusterSecret = cfg.ClusterSecret
	serverConfig.ControlConfig.DataDir = cfg.DataDir
	serverConfig.ControlConfig.KubeConfigOutput = cfg.KubeConfigOutput
	serverConfig.ControlConfig.KubeConfigMode = cfg.KubeConfigMode
	serverConfig.ControlConfig.NoScheduler = cfg.DisableScheduler
	serverConfig.Rootless = cfg.Rootless
	serverConfig.TLSConfig.HTTPSPort = cfg.HTTPSPort
	serverConfig.TLSConfig.HTTPPort = cfg.HTTPPort
	for _, san := range knownIPs(cfg.TLSSan) {
		addr := net2.ParseIP(san)
		if addr != nil {
			serverConfig.TLSConfig.KnownIPs = append(serverConfig.TLSConfig.KnownIPs, san)
		} else {
			serverConfig.TLSConfig.Domains = append(serverConfig.TLSConfig.Domains, san)
		}
	}
	serverConfig.TLSConfig.BindAddress = cfg.BindAddress
	serverConfig.ControlConfig.HTTPSPort = cfg.HTTPSPort
	serverConfig.ControlConfig.ExtraAPIArgs = cfg.ExtraAPIArgs
	serverConfig.ControlConfig.ExtraControllerArgs = cfg.ExtraControllerArgs
	serverConfig.ControlConfig.ExtraSchedulerAPIArgs = cfg.ExtraSchedulerArgs
	serverConfig.ControlConfig.ClusterDomain = cfg.ClusterDomain
	serverConfig.ControlConfig.Storage.Endpoint = cfg.StorageEndpoint
	serverConfig.ControlConfig.Storage.CAFile = cfg.StorageCAFile
	serverConfig.ControlConfig.Storage.CertFile = cfg.StorageCertFile
	serverConfig.ControlConfig.Storage.KeyFile = cfg.StorageKeyFile
	serverConfig.ControlConfig.AdvertiseIP = cfg.AdvertiseIP
	serverConfig.ControlConfig.AdvertisePort = cfg.AdvertisePort
	serverConfig.ControlConfig.BootstrapReadOnly = !cfg.StoreBootstrap

	if cmds.AgentConfig.FlannelIface != "" && cmds.AgentConfig.NodeIP == "" {
		cmds.AgentConfig.NodeIP = netutil.GetIPFromInterface(cmds.AgentConfig.FlannelIface)
	}

	if serverConfig.ControlConfig.AdvertiseIP == "" && cmds.AgentConfig.NodeIP != "" {
		serverConfig.ControlConfig.AdvertiseIP = cmds.AgentConfig.NodeIP
	}
	if serverConfig.ControlConfig.AdvertiseIP != "" {
		serverConfig.TLSConfig.KnownIPs = append(serverConfig.TLSConfig.KnownIPs, serverConfig.ControlConfig.AdvertiseIP)
	}

	_, serverConfig.ControlConfig.ClusterIPRange, err = net2.ParseCIDR(cfg.ClusterCIDR)
	if err != nil {
		return errors.Wrapf(err, "Invalid CIDR %s: %v", cfg.ClusterCIDR, err)
	}
	_, serverConfig.ControlConfig.ServiceIPRange, err = net2.ParseCIDR(cfg.ServiceCIDR)
	if err != nil {
		return errors.Wrapf(err, "Invalid CIDR %s: %v", cfg.ServiceCIDR, err)
	}

	_, apiServerServiceIP, err := master.DefaultServiceIPRange(*serverConfig.ControlConfig.ServiceIPRange)
	if err != nil {
		return err
	}
	serverConfig.TLSConfig.KnownIPs = append(serverConfig.TLSConfig.KnownIPs, apiServerServiceIP.String())

	// If cluster-dns CLI arg is not set, we set ClusterDNS address to be ServiceCIDR network + 10,
	// i.e. when you set service-cidr to 192.168.0.0/16 and don't provide cluster-dns, it will be set to 192.168.0.10
	if cfg.ClusterDNS == "" {
		serverConfig.ControlConfig.ClusterDNS = make(net2.IP, 4)
		copy(serverConfig.ControlConfig.ClusterDNS, serverConfig.ControlConfig.ServiceIPRange.IP.To4())
		serverConfig.ControlConfig.ClusterDNS[3] = 10
	} else {
		serverConfig.ControlConfig.ClusterDNS = net2.ParseIP(cfg.ClusterDNS)
	}

	for _, noDeploy := range app.StringSlice("no-deploy") {
		if noDeploy == "servicelb" {
			serverConfig.DisableServiceLB = true
			continue
		}

		if !strings.HasSuffix(noDeploy, ".yaml") {
			noDeploy = noDeploy + ".yaml"
		}
		serverConfig.ControlConfig.Skips = append(serverConfig.ControlConfig.Skips, noDeploy)
	}

	logrus.Info("Starting k3s ", app.App.Version)
	notifySocket := os.Getenv("NOTIFY_SOCKET")
	os.Unsetenv("NOTIFY_SOCKET")

	ctx := signals.SetupSignalHandler(context.Background())
	certs, err := server.StartServer(ctx, &serverConfig)
	if err != nil {
		return err
	}

	logrus.Info("k3s is up and running")
	if notifySocket != "" {
		os.Setenv("NOTIFY_SOCKET", notifySocket)
		systemd.SdNotify(true, "READY=1\n")
	}

	if cfg.DisableAgent {
		<-ctx.Done()
		return nil
	}
	ip := serverConfig.TLSConfig.BindAddress
	if ip == "" {
		ip = "127.0.0.1"
	}
	url := fmt.Sprintf("https://%s:%d", ip, serverConfig.TLSConfig.HTTPSPort)
	token := server.FormatToken(serverConfig.ControlConfig.Runtime.NodeToken, certs)

	agentConfig := cmds.AgentConfig
	agentConfig.Debug = app.GlobalBool("bool")
	agentConfig.DataDir = filepath.Dir(serverConfig.ControlConfig.DataDir)
	agentConfig.ServerURL = url
	agentConfig.Token = token
	agentConfig.Labels = append(agentConfig.Labels, "node-role.kubernetes.io/master=true")
	agentConfig.DisableLoadBalancer = true

	return agent.Run(ctx, agentConfig)
}

func knownIPs(ips []string) []string {
	ips = append(ips, "127.0.0.1")
	ip, err := net.ChooseHostInterface()
	if err == nil {
		ips = append(ips, ip.String())
	}
	return ips
}
