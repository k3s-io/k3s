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
	"github.com/k3s-io/k3s/pkg/agent"
	"github.com/k3s-io/k3s/pkg/agent/loadbalancer"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/netutil"
	"github.com/k3s-io/k3s/pkg/rootless"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/token"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	kubeapiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/kubernetes/pkg/controlplane"
	utilsnet "k8s.io/utils/net"

	_ "github.com/go-sql-driver/mysql" // ensure we have mysql
	_ "github.com/lib/pq"              // ensure we have postgres
	_ "github.com/mattn/go-sqlite3"    // ensure we have sqlite
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

	// If the agent is enabled, evacuate cgroup v2 before doing anything else that may fork.
	// If the agent is disabled, we don't need to bother doing this as it is only the kubelet
	// that cares about cgroups.
	if !cfg.DisableAgent {
		if err := cmds.EvacuateCgroup2(); err != nil {
			return err
		}
	}

	// Initialize logging, and subprocess reaping if necessary.
	// Log output redirection and subprocess reaping both require forking.
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
		if !cfg.DisableAgent {
			if err := rootless.Rootless(dataDir); err != nil {
				return err
			}
		}
	}

	if cfg.Token == "" && cfg.ClusterSecret != "" {
		cfg.Token = cfg.ClusterSecret
	}

	agentReady := make(chan struct{})

	serverConfig := server.Config{}
	serverConfig.DisableAgent = cfg.DisableAgent
	serverConfig.ControlConfig.Runtime = &config.ControlRuntime{AgentReady: agentReady}
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
	serverConfig.ServiceLBNamespace = cfg.ServiceLBNamespace
	serverConfig.ControlConfig.SANs = cfg.TLSSan
	serverConfig.ControlConfig.BindAddress = cfg.BindAddress
	serverConfig.ControlConfig.SupervisorPort = cfg.SupervisorPort
	serverConfig.ControlConfig.HTTPSPort = cfg.HTTPSPort
	serverConfig.ControlConfig.APIServerPort = cfg.APIServerPort
	serverConfig.ControlConfig.APIServerBindAddress = cfg.APIServerBindAddress
	serverConfig.ControlConfig.EnablePProf = cfg.EnablePProf
	serverConfig.ControlConfig.ExtraAPIArgs = cfg.ExtraAPIArgs
	serverConfig.ControlConfig.ExtraControllerArgs = cfg.ExtraControllerArgs
	serverConfig.ControlConfig.ExtraEtcdArgs = cfg.ExtraEtcdArgs
	serverConfig.ControlConfig.ExtraSchedulerAPIArgs = cfg.ExtraSchedulerArgs
	serverConfig.ControlConfig.ClusterDomain = cfg.ClusterDomain
	serverConfig.ControlConfig.Datastore.Endpoint = cfg.DatastoreEndpoint
	serverConfig.ControlConfig.Datastore.BackendTLSConfig.CAFile = cfg.DatastoreCAFile
	serverConfig.ControlConfig.Datastore.BackendTLSConfig.CertFile = cfg.DatastoreCertFile
	serverConfig.ControlConfig.Datastore.BackendTLSConfig.KeyFile = cfg.DatastoreKeyFile
	serverConfig.ControlConfig.AdvertiseIP = cfg.AdvertiseIP
	serverConfig.ControlConfig.AdvertisePort = cfg.AdvertisePort
	serverConfig.ControlConfig.FlannelBackend = cfg.FlannelBackend
	serverConfig.ControlConfig.FlannelIPv6Masq = cfg.FlannelIPv6Masq
	serverConfig.ControlConfig.EgressSelectorMode = cfg.EgressSelectorMode
	serverConfig.ControlConfig.ExtraCloudControllerArgs = cfg.ExtraCloudControllerArgs
	serverConfig.ControlConfig.DisableCCM = cfg.DisableCCM
	serverConfig.ControlConfig.DisableNPC = cfg.DisableNPC
	serverConfig.ControlConfig.DisableHelmController = cfg.DisableHelmController
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
		serverConfig.ControlConfig.EtcdSnapshotCompress = cfg.EtcdSnapshotCompress
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
		return errors.New("invalid flag use; --cluster-reset required with --cluster-reset-restore-path")
	}

	serverConfig.ControlConfig.ClusterReset = cfg.ClusterReset
	serverConfig.ControlConfig.ClusterResetRestorePath = cfg.ClusterResetRestorePath
	serverConfig.ControlConfig.SystemDefaultRegistry = cfg.SystemDefaultRegistry

	if serverConfig.ControlConfig.SupervisorPort == 0 {
		serverConfig.ControlConfig.SupervisorPort = serverConfig.ControlConfig.HTTPSPort
	}

	if serverConfig.ControlConfig.DisableETCD && serverConfig.ControlConfig.JoinURL == "" {
		return errors.New("invalid flag use; --server is required with --disable-etcd")
	}

	if serverConfig.ControlConfig.DisableAPIServer {
		// Servers without a local apiserver need to connect to the apiserver via the proxy load-balancer.
		serverConfig.ControlConfig.APIServerPort = cmds.AgentConfig.LBServerPort
		// If the supervisor and externally-facing apiserver are not on the same port, the proxy will
		// have a separate load-balancer for the apiserver that we need to use instead.
		if serverConfig.ControlConfig.SupervisorPort != serverConfig.ControlConfig.HTTPSPort {
			serverConfig.ControlConfig.APIServerPort = cmds.AgentConfig.LBServerPort - 1
		}
	}

	if cmds.AgentConfig.FlannelIface != "" && len(cmds.AgentConfig.NodeIP) == 0 {
		cmds.AgentConfig.NodeIP.Set(netutil.GetIPFromInterface(cmds.AgentConfig.FlannelIface))
	}

	if serverConfig.ControlConfig.PrivateIP == "" && len(cmds.AgentConfig.NodeIP) != 0 {
		// ignoring the error here is fine since etcd will fall back to the interface's IPv4 address
		serverConfig.ControlConfig.PrivateIP, _, _ = util.GetFirstString(cmds.AgentConfig.NodeIP)
	}

	// if not set, try setting advertise-ip from agent node-external-ip
	if serverConfig.ControlConfig.AdvertiseIP == "" && len(cmds.AgentConfig.NodeExternalIP) != 0 {
		serverConfig.ControlConfig.AdvertiseIP, _, _ = util.GetFirstString(cmds.AgentConfig.NodeExternalIP)
	}

	// if not set, try setting advertise-ip from agent node-ip
	if serverConfig.ControlConfig.AdvertiseIP == "" && len(cmds.AgentConfig.NodeIP) != 0 {
		serverConfig.ControlConfig.AdvertiseIP, _, _ = util.GetFirstString(cmds.AgentConfig.NodeIP)
	}

	// if we ended up with any advertise-ips, ensure they're added to the SAN list;
	// note that kube-apiserver does not support dual-stack advertise-ip as of 1.21.0:
	/// https://github.com/kubernetes/kubeadm/issues/1612#issuecomment-772583989
	if serverConfig.ControlConfig.AdvertiseIP != "" {
		serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, serverConfig.ControlConfig.AdvertiseIP)
	}

	// Ensure that we add the localhost name/ip and node name/ip to the SAN list. This list is shared by the
	// certs for the supervisor, kube-apiserver cert, and etcd. DNS entries for the in-cluster kubernetes
	// service endpoint are added later when the certificates are created.
	nodeName, nodeIPs, err := util.GetHostnameAndIPs(cmds.AgentConfig.NodeName, cmds.AgentConfig.NodeIP)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.ServerNodeName = nodeName
	serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, "127.0.0.1", "::1", "localhost", nodeName)
	for _, ip := range nodeIPs {
		serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, ip.String())
	}

	// configure ClusterIPRanges
	_, _, IPv6only, _ := util.GetFirstIP(nodeIPs)
	if len(cmds.ServerConfig.ClusterCIDR) == 0 {
		clusterCIDR := "10.42.0.0/16"
		if IPv6only {
			clusterCIDR = "fd00:42::/56"
		}
		cmds.ServerConfig.ClusterCIDR.Set(clusterCIDR)
	}
	for _, cidr := range cmds.ServerConfig.ClusterCIDR {
		for _, v := range strings.Split(cidr, ",") {
			_, parsed, err := net.ParseCIDR(v)
			if err != nil {
				return errors.Wrapf(err, "invalid cluster-cidr %s", v)
			}
			serverConfig.ControlConfig.ClusterIPRanges = append(serverConfig.ControlConfig.ClusterIPRanges, parsed)
		}
	}

	// set ClusterIPRange to the first IPv4 block, for legacy clients
	// unless only IPv6 range given
	clusterIPRange, err := util.GetFirstNet(serverConfig.ControlConfig.ClusterIPRanges)
	if err != nil {
		return errors.Wrap(err, "cannot configure IPv4/IPv6 cluster-cidr")
	}
	serverConfig.ControlConfig.ClusterIPRange = clusterIPRange

	// configure ServiceIPRanges
	if len(cmds.ServerConfig.ServiceCIDR) == 0 {
		serviceCIDR := "10.43.0.0/16"
		if IPv6only {
			serviceCIDR = "fd00:43::/112"
		}
		cmds.ServerConfig.ServiceCIDR.Set(serviceCIDR)
	}
	for _, cidr := range cmds.ServerConfig.ServiceCIDR {
		for _, v := range strings.Split(cidr, ",") {
			_, parsed, err := net.ParseCIDR(v)
			if err != nil {
				return errors.Wrapf(err, "invalid service-cidr %s", v)
			}
			serverConfig.ControlConfig.ServiceIPRanges = append(serverConfig.ControlConfig.ServiceIPRanges, parsed)
		}
	}

	// set ServiceIPRange to the first IPv4 block, for legacy clients
	// unless only IPv6 range given
	serviceIPRange, err := util.GetFirstNet(serverConfig.ControlConfig.ServiceIPRanges)
	if err != nil {
		return errors.Wrap(err, "cannot configure IPv4/IPv6 service-cidr")
	}
	serverConfig.ControlConfig.ServiceIPRange = serviceIPRange

	serverConfig.ControlConfig.ServiceNodePortRange, err = utilnet.ParsePortRange(cfg.ServiceNodePortRange)
	if err != nil {
		return errors.Wrapf(err, "invalid port range %s", cfg.ServiceNodePortRange)
	}

	// the apiserver service does not yet support dual-stack operation
	_, apiServerServiceIP, err := controlplane.ServiceIPRange(*serverConfig.ControlConfig.ServiceIPRange)
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, apiServerServiceIP.String())

	// If cluster-dns CLI arg is not set, we set ClusterDNS address to be the first IPv4 ServiceCIDR network + 10,
	// i.e. when you set service-cidr to 192.168.0.0/16 and don't provide cluster-dns, it will be set to 192.168.0.10
	// If there are no IPv4 ServiceCIDRs, an IPv6 ServiceCIDRs will be used.
	// If neither of IPv4 or IPv6 are found an error is raised.
	if len(cmds.ServerConfig.ClusterDNS) == 0 {
		clusterDNS, err := utilsnet.GetIndexedIP(serverConfig.ControlConfig.ServiceIPRange, 10)
		if err != nil {
			return errors.Wrap(err, "cannot configure default cluster-dns address")
		}
		serverConfig.ControlConfig.ClusterDNS = clusterDNS
		serverConfig.ControlConfig.ClusterDNSs = []net.IP{serverConfig.ControlConfig.ClusterDNS}
	} else {
		for _, ip := range cmds.ServerConfig.ClusterDNS {
			for _, v := range strings.Split(ip, ",") {
				parsed := net.ParseIP(v)
				if parsed == nil {
					return fmt.Errorf("invalid cluster-dns address %s", v)
				}
				serverConfig.ControlConfig.ClusterDNSs = append(serverConfig.ControlConfig.ClusterDNSs, parsed)
			}
		}
		// Set ClusterDNS to the first IPv4 address, for legacy clients
		// unless only IPv6 range given
		clusterDNS, _, _, err := util.GetFirstIP(serverConfig.ControlConfig.ClusterDNSs)
		if err != nil {
			return errors.Wrap(err, "cannot configure IPv4/IPv6 cluster-dns address")
		}
		serverConfig.ControlConfig.ClusterDNS = clusterDNS
	}

	if err := validateNetworkConfiguration(serverConfig); err != nil {
		return err
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
		return errors.Wrap(err, "invalid tls-min-version")
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
		return errors.Wrap(err, "invalid tls-cipher-suites")
	}

	// If performing a cluster reset, make sure control-plane components are
	// disabled so we only perform a reset or restore and bail out.
	if cfg.ClusterReset {
		serverConfig.ControlConfig.ClusterInit = true
		serverConfig.ControlConfig.DisableAPIServer = true
		serverConfig.ControlConfig.DisableControllerManager = true
		serverConfig.ControlConfig.DisableScheduler = true
		serverConfig.ControlConfig.DisableCCM = true

		// If the supervisor and apiserver are on the same port, everything is running embedded
		// and we don't need the kubelet or containerd up to perform a cluster reset.
		if serverConfig.ControlConfig.SupervisorPort == serverConfig.ControlConfig.HTTPSPort {
			cfg.DisableAgent = true
		}

		dataDir, err := datadir.LocalHome(cfg.DataDir, false)
		if err != nil {
			return err
		}
		// delete local loadbalancers state for apiserver and supervisor servers
		loadbalancer.ResetLoadBalancer(filepath.Join(dataDir, "agent"), loadbalancer.SupervisorServiceName)
		loadbalancer.ResetLoadBalancer(filepath.Join(dataDir, "agent"), loadbalancer.APIServerServiceName)

		if cfg.ClusterResetRestorePath != "" {
			// at this point we're doing a restore. Check to see if we've
			// passed in a token and if not, check if the token file exists.
			// If it doesn't, return an error indicating the token is necessary.
			if cfg.Token == "" {
				tokenFile := filepath.Join(dataDir, "server", "token")
				if _, err := os.Stat(tokenFile); err != nil {
					if os.IsNotExist(err) {
						return errors.New(tokenFile + " does not exist, please pass --token to complete the restoration")
					}
				}
			}
		}
	}

	logrus.Info("Starting " + version.Program + " " + app.App.Version)

	notifySocket := os.Getenv("NOTIFY_SOCKET")
	os.Unsetenv("NOTIFY_SOCKET")

	ctx := signals.SetupSignalContext()

	if err := server.StartServer(ctx, &serverConfig, cfg); err != nil {
		return err
	}

	go func() {
		if !serverConfig.ControlConfig.DisableAPIServer {
			<-serverConfig.ControlConfig.Runtime.APIServerReady
			logrus.Info("Kube API server is now running")
			serverConfig.ControlConfig.Runtime.StartupHooksWg.Wait()
		}
		if !serverConfig.ControlConfig.DisableETCD {
			<-serverConfig.ControlConfig.Runtime.ETCDReady
			logrus.Info("ETCD server is now running")
		}

		logrus.Info(version.Program + " is up and running")
		os.Setenv("NOTIFY_SOCKET", notifySocket)
		systemd.SdNotify(true, "READY=1\n")
	}()

	url := fmt.Sprintf("https://%s:%d", serverConfig.ControlConfig.BindAddressOrLoopback(false), serverConfig.ControlConfig.SupervisorPort)
	token, err := clientaccess.FormatToken(serverConfig.ControlConfig.Runtime.AgentToken, serverConfig.ControlConfig.Runtime.ServerCA)
	if err != nil {
		return err
	}

	agentConfig := cmds.AgentConfig
	agentConfig.AgentReady = agentReady
	agentConfig.Debug = app.GlobalBool("debug")
	agentConfig.DataDir = filepath.Dir(serverConfig.ControlConfig.DataDir)
	agentConfig.ServerURL = url
	agentConfig.Token = token
	agentConfig.DisableLoadBalancer = !serverConfig.ControlConfig.DisableAPIServer
	agentConfig.DisableServiceLB = serverConfig.DisableServiceLB
	agentConfig.ETCDAgent = serverConfig.ControlConfig.DisableAPIServer
	agentConfig.ClusterReset = serverConfig.ControlConfig.ClusterReset
	agentConfig.Rootless = cfg.Rootless

	if agentConfig.Rootless {
		// let agent specify Rootless kubelet flags, but not unshare twice
		agentConfig.RootlessAlreadyUnshared = true
	}

	if serverConfig.ControlConfig.DisableAPIServer {
		if cfg.ServerURL == "" {
			// If this node is the initial member of the cluster and is not hosting an apiserver,
			// always bootstrap the agent off local supervisor, and go through the process of reading
			// apiserver endpoints from etcd and blocking further startup until one is available.
			// This ensures that we don't end up in a chicken-and-egg situation on cluster restarts,
			// where the loadbalancer is routing traffic to existing apiservers, but the apiservers
			// are non-functional because they're waiting for us to start etcd.
			loadbalancer.ResetLoadBalancer(filepath.Join(agentConfig.DataDir, "agent"), loadbalancer.SupervisorServiceName)
		} else {
			// If this is a secondary member of the cluster and is not hosting an apiserver,
			// bootstrap the agent off the existing supervisor, instead of bootstrapping locally.
			agentConfig.ServerURL = cfg.ServerURL
		}
		// initialize the apiAddress Channel for receiving the api address from etcd
		agentConfig.APIAddressCh = make(chan []string)
		go getAPIAddressFromEtcd(ctx, serverConfig, agentConfig)
	}

	if cfg.DisableAgent {
		agentConfig.ContainerRuntimeEndpoint = "/dev/null"
		return agent.RunStandalone(ctx, agentConfig)
	}

	return agent.Run(ctx, agentConfig)
}

// validateNetworkConfig ensures that the network configuration values make sense.
func validateNetworkConfiguration(serverConfig server.Config) error {
	// Dual-stack operation requires fairly extensive manual configuration at the moment - do some
	// preflight checks to make sure that the user isn't trying to use flannel/npc, or trying to
	// enable dual-stack DNS (which we don't currently support since it's not easy to template)
	dualDNS, err := utilsnet.IsDualStackIPs(serverConfig.ControlConfig.ClusterDNSs)
	if err != nil {
		return errors.Wrap(err, "failed to validate cluster-dns")
	}

	if dualDNS == true {
		return errors.New("dual-stack cluster-dns is not supported")
	}

	switch serverConfig.ControlConfig.EgressSelectorMode {
	case config.EgressSelectorModeAgent, config.EgressSelectorModeCluster,
		config.EgressSelectorModeDisabled, config.EgressSelectorModePod:
	default:
		return fmt.Errorf("invalid egress-selector-mode %s", serverConfig.ControlConfig.EgressSelectorMode)
	}

	return nil
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

func getAPIAddressFromEtcd(ctx context.Context, serverConfig server.Config, agentConfig cmds.Agent) {
	defer close(agentConfig.APIAddressCh)
	for {
		toCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		serverAddresses, err := etcd.GetAPIServerURLsFromETCD(toCtx, &serverConfig.ControlConfig)
		if err == nil && len(serverAddresses) > 0 {
			agentConfig.APIAddressCh <- serverAddresses
			break
		}
		if !errors.Is(err, etcd.ErrAddressNotSet) {
			logrus.Warnf("Failed to get apiserver address from etcd: %v", err)
		}
		<-toCtx.Done()
	}
}
