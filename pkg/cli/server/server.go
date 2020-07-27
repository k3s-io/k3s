package server

import (
	"context"
	"fmt"
	net2 "net"
	"os"
	"path/filepath"
	"strings"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/erikdubbelboer/gspt"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/netutil"
	"github.com/rancher/k3s/pkg/rootless"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/k3s/pkg/token"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/spur/cli"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/net"
	kubeapiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/kubernetes/pkg/master"

	_ "github.com/go-sql-driver/mysql" // ensure we have mysql
	_ "github.com/lib/pq"              // ensure we have postgres
	_ "github.com/mattn/go-sqlite3"    // ensure we have sqlite
)

func Run(app *cli.Context) error {
	return run(app, &cmds.ServerConfig)
}

func run(app *cli.Context, cfg *cmds.Server) error {
	var (
		err error
	)

	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	gspt.SetProcTitle(os.Args[0] + " server")

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

	if cfg.Token == "" && cfg.ClusterSecret != "" {
		cfg.Token = cfg.ClusterSecret
	}

	serverConfig := server.Config{}
	serverConfig.DisableAgent = cfg.DisableAgent
	serverConfig.ControlConfig.Token = cfg.Token
	serverConfig.ControlConfig.AgentToken = cfg.AgentToken
	serverConfig.ControlConfig.JoinURL = cfg.ServerURL
	if cfg.AgentTokenFile != "" {
		serverConfig.ControlConfig.AgentToken, err = token.ReadFile(cfg.AgentTokenFile)
		if err != nil {
			return err
		}
	}
	if cfg.TokenFile != "" {
		serverConfig.ControlConfig.Token, err = token.ReadFile(cfg.TokenFile)
		if err != nil {
			return err
		}
	}
	serverConfig.ControlConfig.DataDir = cfg.DataDir
	serverConfig.ControlConfig.KubeConfigOutput = cfg.KubeConfigOutput
	serverConfig.ControlConfig.KubeConfigMode = cfg.KubeConfigMode
	serverConfig.ControlConfig.NoScheduler = cfg.DisableScheduler
	serverConfig.Rootless = cfg.Rootless
	serverConfig.ControlConfig.SANs = knownIPs(cfg.TLSSan)
	serverConfig.ControlConfig.BindAddress = cfg.BindAddress
	serverConfig.ControlConfig.SupervisorPort = cfg.SupervisorPort
	serverConfig.ControlConfig.HTTPSPort = cfg.HTTPSPort
	serverConfig.ControlConfig.APIServerPort = cfg.APIServerPort
	serverConfig.ControlConfig.APIServerBindAddress = cfg.APIServerBindAddress
	serverConfig.ControlConfig.ExtraAPIArgs = cfg.ExtraAPIArgs
	serverConfig.ControlConfig.ExtraControllerArgs = cfg.ExtraControllerArgs
	serverConfig.ControlConfig.ExtraSchedulerAPIArgs = cfg.ExtraSchedulerArgs
	serverConfig.ControlConfig.ClusterDomain = cfg.ClusterDomain
	serverConfig.ControlConfig.Datastore.Endpoint = cfg.DatastoreEndpoint
	serverConfig.ControlConfig.Datastore.CAFile = cfg.DatastoreCAFile
	serverConfig.ControlConfig.Datastore.CertFile = cfg.DatastoreCertFile
	serverConfig.ControlConfig.Datastore.KeyFile = cfg.DatastoreKeyFile
	serverConfig.ControlConfig.AdvertiseIP = cfg.AdvertiseIP
	serverConfig.ControlConfig.AdvertisePort = cfg.AdvertisePort
	serverConfig.ControlConfig.FlannelBackend = cfg.FlannelBackend
	serverConfig.ControlConfig.ExtraCloudControllerArgs = cfg.ExtraCloudControllerArgs
	serverConfig.ControlConfig.DisableCCM = cfg.DisableCCM
	serverConfig.ControlConfig.DisableNPC = cfg.DisableNPC
	serverConfig.ControlConfig.DisableKubeProxy = cfg.DisableKubeProxy
	serverConfig.ControlConfig.ClusterInit = cfg.ClusterInit
	serverConfig.ControlConfig.ClusterReset = cfg.ClusterReset
	serverConfig.ControlConfig.EncryptSecrets = cfg.EncryptSecrets

	if serverConfig.ControlConfig.SupervisorPort == 0 {
		serverConfig.ControlConfig.SupervisorPort = serverConfig.ControlConfig.HTTPSPort
	}

	if cmds.AgentConfig.FlannelIface != "" && cmds.AgentConfig.NodeIP == "" {
		cmds.AgentConfig.NodeIP = netutil.GetIPFromInterface(cmds.AgentConfig.FlannelIface)
	}

	if serverConfig.ControlConfig.AdvertiseIP == "" && cmds.AgentConfig.NodeExternalIP != "" {
		serverConfig.ControlConfig.AdvertiseIP = cmds.AgentConfig.NodeExternalIP
	}
	if serverConfig.ControlConfig.AdvertiseIP == "" && cmds.AgentConfig.NodeIP != "" {
		serverConfig.ControlConfig.AdvertiseIP = cmds.AgentConfig.NodeIP
	}
	if serverConfig.ControlConfig.AdvertiseIP != "" {
		serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, serverConfig.ControlConfig.AdvertiseIP)
	}

	_, serverConfig.ControlConfig.ClusterIPRange, err = net2.ParseCIDR(cfg.ClusterCIDR)
	if err != nil {
		return errors.Wrapf(err, "Invalid CIDR %s: %v", cfg.ClusterCIDR, err)
	}
	_, serverConfig.ControlConfig.ServiceIPRange, err = net2.ParseCIDR(cfg.ServiceCIDR)
	if err != nil {
		return errors.Wrapf(err, "Invalid CIDR %s: %v", cfg.ServiceCIDR, err)
	}

	_, apiServerServiceIP, err := master.ServiceIPRange(*serverConfig.ControlConfig.ServiceIPRange)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, apiServerServiceIP.String())

	// If cluster-dns CLI arg is not set, we set ClusterDNS address to be ServiceCIDR network + 10,
	// i.e. when you set service-cidr to 192.168.0.0/16 and don't provide cluster-dns, it will be set to 192.168.0.10
	if cfg.ClusterDNS == "" {
		serverConfig.ControlConfig.ClusterDNS = make(net2.IP, 4)
		copy(serverConfig.ControlConfig.ClusterDNS, serverConfig.ControlConfig.ServiceIPRange.IP.To4())
		serverConfig.ControlConfig.ClusterDNS[3] = 10
	} else {
		serverConfig.ControlConfig.ClusterDNS = net2.ParseIP(cfg.ClusterDNS)
	}

	if cfg.DefaultLocalStoragePath == "" {
		dataDir, err := datadir.LocalHome(cfg.DataDir, false)
		if err != nil {
			return err
		}
		serverConfig.ControlConfig.DefaultLocalStoragePath = filepath.Join(dataDir, "/storage")
	} else {
		serverConfig.ControlConfig.DefaultLocalStoragePath = cfg.DefaultLocalStoragePath
	}

	serverConfig.ControlConfig.Skips = map[string]bool{}
	for _, noDeploy := range app.StringSlice("no-deploy") {
		for _, v := range strings.Split(noDeploy, ",") {
			v = strings.TrimSpace(v)
			serverConfig.ControlConfig.Skips[v] = true
		}
	}
	serverConfig.ControlConfig.Disables = map[string]bool{}
	for _, disable := range app.StringSlice("disable") {
		for _, v := range strings.Split(disable, ",") {
			v = strings.TrimSpace(v)
			serverConfig.ControlConfig.Skips[v] = true
			serverConfig.ControlConfig.Disables[v] = true
		}
	}
	if serverConfig.ControlConfig.Skips["servicelb"] {
		serverConfig.DisableServiceLB = true
	}

	if serverConfig.ControlConfig.DisableCCM {
		serverConfig.ControlConfig.Skips["ccm"] = true
		serverConfig.ControlConfig.Disables["ccm"] = true
	}

	TLSMinVersion := getArgValueFromList("tls-min-version", cfg.ExtraAPIArgs)
	serverConfig.ControlConfig.TLSMinVersion, err = kubeapiserverflag.TLSVersion(TLSMinVersion)
	if err != nil {
		return errors.Wrapf(err, "Invalid TLS Version %s: %v", TLSMinVersion, err)
	}

	// TLS config based on mozilla ssl-config generator
	// https://ssl-config.mozilla.org/#server=golang&version=1.13.6&config=intermediate&guideline=5.4
	// Need to disable the TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256 Cipher for TLS1.2
	TLSCipherSuites := []string{getArgValueFromList("tls-cipher-suites", cfg.ExtraAPIArgs)}
	if len(TLSCipherSuites) == 0 || TLSCipherSuites[0] == "" {
		TLSCipherSuites = []string{
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		}
	}
	serverConfig.ControlConfig.TLSCipherSuites, err = kubeapiserverflag.TLSCipherSuites(TLSCipherSuites)
	if err != nil {
		return errors.Wrapf(err, "Invalid TLS Cipher Suites %s: %v", TLSCipherSuites, err)
	}

	logrus.Info("Starting "+version.Program+" ", app.App.Version)
	notifySocket := os.Getenv("NOTIFY_SOCKET")
	os.Unsetenv("NOTIFY_SOCKET")

	ctx := signals.SetupSignalHandler(context.Background())
	if err := server.StartServer(ctx, &serverConfig); err != nil {
		return err
	}

	go func() {
		<-serverConfig.ControlConfig.Runtime.APIServerReady
		logrus.Info("" + version.Program + " is up and running")
		if notifySocket != "" {
			os.Setenv("NOTIFY_SOCKET", notifySocket)
			systemd.SdNotify(true, "READY=1\n")
		}
	}()

	if cfg.DisableAgent {
		<-ctx.Done()
		return nil
	}

	ip := serverConfig.ControlConfig.BindAddress
	if ip == "" {
		ip = "127.0.0.1"
	}

	url := fmt.Sprintf("https://%s:%d", ip, serverConfig.ControlConfig.SupervisorPort)
	token, err := server.FormatToken(serverConfig.ControlConfig.Runtime.AgentToken, serverConfig.ControlConfig.Runtime.ServerCA)
	if err != nil {
		return err
	}

	agentConfig := cmds.AgentConfig
	agentConfig.Debug = app.Bool("debug")
	agentConfig.DataDir = filepath.Dir(serverConfig.ControlConfig.DataDir)
	agentConfig.ServerURL = url
	agentConfig.Token = token
	agentConfig.DisableLoadBalancer = true
	agentConfig.Rootless = cfg.Rootless
	if agentConfig.Rootless {
		// let agent specify Rootless kubelet flags, but not unshare twice
		agentConfig.RootlessAlreadyUnshared = true
	}

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

func getArgValueFromList(searchArg string, argList []string) string {
	var value string
	for _, arg := range argList {
		splitArg := strings.SplitN(arg, "=", 2)
		if splitArg[0] == searchArg {
			value = splitArg[1]
			// break if we found our value
			break
		}
	}
	return value
}
