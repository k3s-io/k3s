package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/erikdubbelboer/gspt"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent"
	"github.com/rancher/k3s/pkg/agent/loadbalancer"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/rancher/k3s/pkg/netutil"
	"github.com/rancher/k3s/pkg/rootless"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/k3s/pkg/token"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	kubeapiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/kubernetes/pkg/controlplane"

	_ "github.com/go-sql-driver/mysql" // ensure we have mysql
	_ "github.com/lib/pq"              // ensure we have postgres
	_ "github.com/mattn/go-sqlite3"    // ensure we have sqlite
)

const (
	lbServerPort = 6444
)

func Run(app *cli.Context) error {
	return run(app, &cmds.ServerConfig, server.CustomControllers{}, server.CustomControllers{})
}

func RunWithControllers(app *cli.Context, leaderControllers server.CustomControllers, controllers server.CustomControllers) error {
	return run(app, &cmds.ServerConfig, leaderControllers, controllers)
}

func run(app *cli.Context, cfg *cmds.Server, leaderControllers server.CustomControllers, controllers server.CustomControllers) error {
	var (
		err error
	)

	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	gspt.SetProcTitle(os.Args[0] + " server")

	// Do init stuff if pid 1.
	// This must be done before InitLogging as that may reexec in order to capture log output
	if err := cmds.HandleInit(); err != nil {
		return err
	}

	if err := cmds.InitLogging(); err != nil {
		return err
	}

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
	serverConfig.ControlConfig.DisableETCD = cfg.DisableETCD
	serverConfig.ControlConfig.DisableAPIServer = cfg.DisableAPIServer
	serverConfig.ControlConfig.DisableScheduler = cfg.DisableScheduler
	serverConfig.ControlConfig.DisableControllerManager = cfg.DisableControllerManager
	serverConfig.ControlConfig.ClusterInit = cfg.ClusterInit
	serverConfig.ControlConfig.EncryptSecrets = cfg.EncryptSecrets
	serverConfig.ControlConfig.EtcdExposeMetrics = cfg.EtcdExposeMetrics
	serverConfig.ControlConfig.EtcdDisableSnapshots = cfg.EtcdDisableSnapshots

	if !cfg.EtcdDisableSnapshots {
		serverConfig.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
		serverConfig.ControlConfig.EtcdSnapshotCron = cfg.EtcdSnapshotCron
		serverConfig.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
		serverConfig.ControlConfig.EtcdSnapshotRetention = cfg.EtcdSnapshotRetention
		serverConfig.ControlConfig.EtcdS3 = cfg.EtcdS3
		serverConfig.ControlConfig.EtcdS3Endpoint = cfg.EtcdS3Endpoint
		serverConfig.ControlConfig.EtcdS3EndpointCA = cfg.EtcdS3EndpointCA
		serverConfig.ControlConfig.EtcdS3SkipSSLVerify = cfg.EtcdS3SkipSSLVerify
		serverConfig.ControlConfig.EtcdS3AccessKey = cfg.EtcdS3AccessKey
		serverConfig.ControlConfig.EtcdS3SecretKey = cfg.EtcdS3SecretKey
		serverConfig.ControlConfig.EtcdS3BucketName = cfg.EtcdS3BucketName
		serverConfig.ControlConfig.EtcdS3Region = cfg.EtcdS3Region
		serverConfig.ControlConfig.EtcdS3Folder = cfg.EtcdS3Folder
		serverConfig.ControlConfig.EtcdS3Insecure = cfg.EtcdS3Insecure
		serverConfig.ControlConfig.EtcdS3Timeout = cfg.EtcdS3Timeout
	} else {
		logrus.Info("ETCD snapshots are disabled")
	}

	if cfg.ClusterResetRestorePath != "" && !cfg.ClusterReset {
		return errors.New("invalid flag use. --cluster-reset required with --cluster-reset-restore-path")
	}

	// make sure components are disabled so we only perform a restore
	// and bail out
	if cfg.ClusterResetRestorePath != "" && cfg.ClusterReset {
		serverConfig.ControlConfig.ClusterInit = true
		serverConfig.ControlConfig.DisableAPIServer = true
		serverConfig.ControlConfig.DisableControllerManager = true
		serverConfig.ControlConfig.DisableScheduler = true
		serverConfig.ControlConfig.DisableCCM = true

		// delete local loadbalancers state for apiserver and supervisor servers
		loadbalancer.ResetLoadBalancer(filepath.Join(cfg.DataDir, "agent"), loadbalancer.SupervisorServiceName)
		loadbalancer.ResetLoadBalancer(filepath.Join(cfg.DataDir, "agent"), loadbalancer.APIServerServiceName)
	}

	serverConfig.ControlConfig.ClusterReset = cfg.ClusterReset
	serverConfig.ControlConfig.ClusterResetRestorePath = cfg.ClusterResetRestorePath

	if serverConfig.ControlConfig.SupervisorPort == 0 {
		serverConfig.ControlConfig.SupervisorPort = serverConfig.ControlConfig.HTTPSPort
	}

	if serverConfig.ControlConfig.DisableAPIServer {
		serverConfig.ControlConfig.APIServerPort = lbServerPort
		if serverConfig.ControlConfig.SupervisorPort != serverConfig.ControlConfig.HTTPSPort {
			serverConfig.ControlConfig.APIServerPort = lbServerPort + 1
		}
	}

	if cmds.AgentConfig.FlannelIface != "" && cmds.AgentConfig.NodeIP == "" {
		cmds.AgentConfig.NodeIP = netutil.GetIPFromInterface(cmds.AgentConfig.FlannelIface)
	}
	if serverConfig.ControlConfig.PrivateIP == "" && cmds.AgentConfig.NodeIP != "" {
		serverConfig.ControlConfig.PrivateIP = cmds.AgentConfig.NodeIP
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

	_, serverConfig.ControlConfig.ClusterIPRange, err = net.ParseCIDR(cfg.ClusterCIDR)
	if err != nil {
		return errors.Wrapf(err, "Invalid CIDR %s: %v", cfg.ClusterCIDR, err)
	}
	_, serverConfig.ControlConfig.ServiceIPRange, err = net.ParseCIDR(cfg.ServiceCIDR)
	if err != nil {
		return errors.Wrapf(err, "Invalid CIDR %s: %v", cfg.ServiceCIDR, err)
	}

	serverConfig.ControlConfig.ServiceNodePortRange, err = utilnet.ParsePortRange(cfg.ServiceNodePortRange)
	if err != nil {
		return errors.Wrapf(err, "Invalid port range %s: %v", cfg.ServiceNodePortRange, err)
	}

	_, apiServerServiceIP, err := controlplane.ServiceIPRange(*serverConfig.ControlConfig.ServiceIPRange)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, apiServerServiceIP.String())

	// If cluster-dns CLI arg is not set, we set ClusterDNS address to be ServiceCIDR network + 10,
	// i.e. when you set service-cidr to 192.168.0.0/16 and don't provide cluster-dns, it will be set to 192.168.0.10
	if cfg.ClusterDNS == "" {
		serverConfig.ControlConfig.ClusterDNS = make(net.IP, 4)
		copy(serverConfig.ControlConfig.ClusterDNS, serverConfig.ControlConfig.ServiceIPRange.IP.To4())
		serverConfig.ControlConfig.ClusterDNS[3] = 10
	} else {
		serverConfig.ControlConfig.ClusterDNS = net.ParseIP(cfg.ClusterDNS)
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

	tlsMinVersionArg := getArgValueFromList("tls-min-version", cfg.ExtraAPIArgs)
	serverConfig.ControlConfig.TLSMinVersion, err = kubeapiserverflag.TLSVersion(tlsMinVersionArg)
	if err != nil {
		return errors.Wrap(err, "Invalid tls-min-version")
	}

	serverConfig.StartupHooks = append(serverConfig.StartupHooks, cfg.StartupHooks...)

	serverConfig.LeaderControllers = append(serverConfig.LeaderControllers, leaderControllers...)
	serverConfig.Controllers = append(serverConfig.Controllers, controllers...)

	// TLS config based on mozilla ssl-config generator
	// https://ssl-config.mozilla.org/#server=golang&version=1.13.6&config=intermediate&guideline=5.4
	// Need to disable the TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256 Cipher for TLS1.2
	tlsCipherSuitesArg := getArgValueFromList("tls-cipher-suites", cfg.ExtraAPIArgs)
	tlsCipherSuites := strings.Split(tlsCipherSuitesArg, ",")
	for i := range tlsCipherSuites {
		tlsCipherSuites[i] = strings.TrimSpace(tlsCipherSuites[i])
	}
	if len(tlsCipherSuites) == 0 || tlsCipherSuites[0] == "" {
		tlsCipherSuites = []string{
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
			"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
		}
	}
	serverConfig.ControlConfig.TLSCipherSuites, err = kubeapiserverflag.TLSCipherSuites(tlsCipherSuites)
	if err != nil {
		return errors.Wrap(err, "Invalid tls-cipher-suites")
	}

	logrus.Info("Starting " + version.Program + " " + app.App.Version)
	notifySocket := os.Getenv("NOTIFY_SOCKET")
	os.Unsetenv("NOTIFY_SOCKET")

	ctx := signals.SetupSignalHandler(context.Background())

	if err := server.StartServer(ctx, &serverConfig); err != nil {
		return err
	}

	go func() {
		if !serverConfig.ControlConfig.DisableAPIServer {
			<-serverConfig.ControlConfig.Runtime.APIServerReady
			logrus.Info("Kube API server is now running")
		} else {
			<-serverConfig.ControlConfig.Runtime.ETCDReady
			logrus.Info("ETCD server is now running")
		}
		logrus.Info(version.Program + " is up and running")
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
	token, err := clientaccess.FormatToken(serverConfig.ControlConfig.Runtime.AgentToken, serverConfig.ControlConfig.Runtime.ServerCA)
	if err != nil {
		return err
	}

	agentConfig := cmds.AgentConfig
	agentConfig.Debug = app.GlobalBool("debug")
	agentConfig.DataDir = filepath.Dir(serverConfig.ControlConfig.DataDir)
	agentConfig.ServerURL = url
	agentConfig.Token = token
	agentConfig.DisableLoadBalancer = !serverConfig.ControlConfig.DisableAPIServer
	agentConfig.ETCDAgent = serverConfig.ControlConfig.DisableAPIServer

	agentConfig.Rootless = cfg.Rootless

	if agentConfig.Rootless {
		// let agent specify Rootless kubelet flags, but not unshare twice
		agentConfig.RootlessAlreadyUnshared = true
	}

	if serverConfig.ControlConfig.DisableAPIServer {
		// setting LBServerPort to a prespecified port to initialize the kubeconfigs with the right address
		agentConfig.LBServerPort = lbServerPort
		// initialize the apiAddress Channel for receiving the api address from etcd
		agentConfig.APIAddressCh = make(chan string, 1)
		setAPIAddressChannel(ctx, &serverConfig, &agentConfig)
		defer close(agentConfig.APIAddressCh)
	}
	return agent.Run(ctx, agentConfig)
}

func knownIPs(ips []string) []string {
	ips = append(ips, "127.0.0.1")
	ip, err := utilnet.ChooseHostInterface()
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

// setAPIAddressChannel will try to get the api address key from etcd and when it succeed it will
// set the APIAddressCh channel with its value, the function works for both k3s and rke2 in case
// of k3s we block returning back to the agent.Run until we get the api address, however in rke2
// the code will not block operation and will run the operation in a goroutine
func setAPIAddressChannel(ctx context.Context, serverConfig *server.Config, agentConfig *cmds.Agent) {
	// start a goroutine to check for the server ip if set from etcd in case of rke2
	if serverConfig.ControlConfig.HTTPSPort != serverConfig.ControlConfig.SupervisorPort {
		go getAPIAddressFromEtcd(ctx, serverConfig, agentConfig)
		return
	}
	getAPIAddressFromEtcd(ctx, serverConfig, agentConfig)
}

func getAPIAddressFromEtcd(ctx context.Context, serverConfig *server.Config, agentConfig *cmds.Agent) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		serverAddress, err := etcd.GetAPIServerURLFromETCD(ctx, &serverConfig.ControlConfig)
		if err == nil {
			agentConfig.ServerURL = "https://" + serverAddress
			agentConfig.APIAddressCh <- agentConfig.ServerURL
			break
		}
		logrus.Warn(err)
	}
}
