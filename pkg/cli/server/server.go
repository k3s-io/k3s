package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	systemd "github.com/coreos/go-systemd/v22/daemon"
	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/agent"
	"github.com/k3s-io/k3s/pkg/agent/https"
	"github.com/k3s-io/k3s/pkg/agent/loadbalancer"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/etcd"
	k3smetrics "github.com/k3s-io/k3s/pkg/metrics"
	"github.com/k3s-io/k3s/pkg/proctitle"
	"github.com/k3s-io/k3s/pkg/profile"
	"github.com/k3s-io/k3s/pkg/rootless"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/k3s-io/k3s/pkg/vpn"
	"github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	kubeapiserverflag "k8s.io/component-base/cli/flag"
	"k8s.io/kubernetes/pkg/controlplane/apiserver/options"
	utilsnet "k8s.io/utils/net"
)

func Run(app *cli.Context) error {
	return run(app, &cmds.ServerConfig, server.CustomControllers{}, server.CustomControllers{})
}

func RunWithControllers(app *cli.Context, leaderControllers server.CustomControllers, controllers server.CustomControllers) error {
	return run(app, &cmds.ServerConfig, leaderControllers, controllers)
}

func run(app *cli.Context, cfg *cmds.Server, leaderControllers server.CustomControllers, controllers server.CustomControllers) error {
	var err error
	// Validate build env
	cmds.MustValidateGolang()

	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	proctitle.SetProcTitle(os.Args[0] + " server")

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
		return fmt.Errorf("server must run as root, or with --rootless and/or --disable-agent")
	}

	if cfg.Rootless {
		dataDir, err := datadir.LocalHome(cfg.DataDir, true)
		if err != nil {
			return err
		}
		cfg.DataDir = dataDir
		if !cfg.DisableAgent {
			dualNode, err := utilsnet.IsDualStackIPStrings(cmds.AgentConfig.NodeIP)
			if err != nil {
				return err
			}
			if err := rootless.Rootless(dataDir, dualNode); err != nil {
				return err
			}
		}
	}

	if cmds.AgentConfig.VPNAuthFile != "" {
		cmds.AgentConfig.VPNAuth, err = util.ReadFile(cmds.AgentConfig.VPNAuthFile)
		if err != nil {
			return err
		}
	}

	// Starts the VPN in the server if config was set up
	if cmds.AgentConfig.VPNAuth != "" {
		err := vpn.StartVPN(cmds.AgentConfig.VPNAuth)
		if err != nil {
			return err
		}
	}

	containerRuntimeReady := make(chan struct{})

	serverConfig := server.Config{}
	serverConfig.DisableAgent = cfg.DisableAgent
	serverConfig.ControlConfig.Runtime = config.NewRuntime(containerRuntimeReady)
	serverConfig.ControlConfig.Token = cfg.Token
	serverConfig.ControlConfig.AgentToken = cfg.AgentToken
	serverConfig.ControlConfig.JoinURL = cfg.ServerURL
	if cfg.AgentTokenFile != "" {
		serverConfig.ControlConfig.AgentToken, err = util.ReadFile(cfg.AgentTokenFile)
		if err != nil {
			return err
		}
	}
	if cfg.TokenFile != "" {
		serverConfig.ControlConfig.Token, err = util.ReadFile(cfg.TokenFile)
		if err != nil {
			return err
		}
	}
	serverConfig.ControlConfig.DataDir = cfg.DataDir
	serverConfig.ControlConfig.KubeConfigOutput = cfg.KubeConfigOutput
	serverConfig.ControlConfig.KubeConfigMode = cfg.KubeConfigMode
	serverConfig.ControlConfig.KubeConfigGroup = cfg.KubeConfigGroup
	serverConfig.ControlConfig.HelmJobImage = cfg.HelmJobImage
	serverConfig.ControlConfig.Rootless = cfg.Rootless
	serverConfig.ControlConfig.ServiceLBNamespace = cfg.ServiceLBNamespace
	serverConfig.ControlConfig.SANs = util.SplitStringSlice(cfg.TLSSan)
	serverConfig.ControlConfig.SANSecurity = cfg.TLSSanSecurity
	serverConfig.ControlConfig.BindAddress = cmds.AgentConfig.BindAddress
	serverConfig.ControlConfig.SupervisorPort = cfg.SupervisorPort
	serverConfig.ControlConfig.HTTPSPort = cfg.HTTPSPort
	serverConfig.ControlConfig.APIServerPort = cfg.APIServerPort
	serverConfig.ControlConfig.APIServerBindAddress = cfg.APIServerBindAddress
	serverConfig.ControlConfig.ExtraAPIArgs = cfg.ExtraAPIArgs
	serverConfig.ControlConfig.ExtraControllerArgs = cfg.ExtraControllerArgs
	serverConfig.ControlConfig.ExtraEtcdArgs = cfg.ExtraEtcdArgs
	serverConfig.ControlConfig.ExtraSchedulerAPIArgs = cfg.ExtraSchedulerArgs
	serverConfig.ControlConfig.ClusterDomain = cfg.ClusterDomain
	serverConfig.ControlConfig.Datastore.NotifyInterval = 5 * time.Second
	serverConfig.ControlConfig.Datastore.Endpoint = cfg.DatastoreEndpoint
	serverConfig.ControlConfig.Datastore.BackendTLSConfig.CAFile = cfg.DatastoreCAFile
	serverConfig.ControlConfig.Datastore.BackendTLSConfig.CertFile = cfg.DatastoreCertFile
	serverConfig.ControlConfig.Datastore.BackendTLSConfig.KeyFile = cfg.DatastoreKeyFile
	serverConfig.ControlConfig.KineTLS = cfg.KineTLS
	serverConfig.ControlConfig.AdvertiseIP = cfg.AdvertiseIP
	serverConfig.ControlConfig.AdvertisePort = cfg.AdvertisePort
	serverConfig.ControlConfig.FlannelBackend = cfg.FlannelBackend
	serverConfig.ControlConfig.FlannelIPv6Masq = cfg.FlannelIPv6Masq
	serverConfig.ControlConfig.FlannelExternalIP = cfg.FlannelExternalIP
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
	serverConfig.ControlConfig.DisableAgent = cfg.DisableAgent
	serverConfig.ControlConfig.EmbeddedRegistry = cfg.EmbeddedRegistry
	serverConfig.ControlConfig.ClusterInit = cfg.ClusterInit
	serverConfig.ControlConfig.EncryptSecrets = cfg.EncryptSecrets
	serverConfig.ControlConfig.EtcdExposeMetrics = cfg.EtcdExposeMetrics
	serverConfig.ControlConfig.EtcdDisableSnapshots = cfg.EtcdDisableSnapshots
	serverConfig.ControlConfig.SupervisorMetrics = cfg.SupervisorMetrics
	serverConfig.ControlConfig.VLevel = cmds.LogConfig.VLevel
	serverConfig.ControlConfig.VModule = cmds.LogConfig.VModule

	if !cfg.EtcdDisableSnapshots || cfg.ClusterReset {
		serverConfig.ControlConfig.EtcdSnapshotCompress = cfg.EtcdSnapshotCompress
		serverConfig.ControlConfig.EtcdSnapshotName = cfg.EtcdSnapshotName
		serverConfig.ControlConfig.EtcdSnapshotCron = cfg.EtcdSnapshotCron
		serverConfig.ControlConfig.EtcdSnapshotDir = cfg.EtcdSnapshotDir
		serverConfig.ControlConfig.EtcdSnapshotRetention = cfg.EtcdSnapshotRetention
		if cfg.EtcdS3 {
			serverConfig.ControlConfig.EtcdS3 = &config.EtcdS3{
				AccessKey:     cfg.EtcdS3AccessKey,
				Bucket:        cfg.EtcdS3BucketName,
				ConfigSecret:  cfg.EtcdS3ConfigSecret,
				Endpoint:      cfg.EtcdS3Endpoint,
				EndpointCA:    cfg.EtcdS3EndpointCA,
				Folder:        cfg.EtcdS3Folder,
				Insecure:      cfg.EtcdS3Insecure,
				Proxy:         cfg.EtcdS3Proxy,
				Region:        cfg.EtcdS3Region,
				SecretKey:     cfg.EtcdS3SecretKey,
				SkipSSLVerify: cfg.EtcdS3SkipSSLVerify,
				Timeout:       metav1.Duration{Duration: cfg.EtcdS3Timeout},
			}
		}
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

	if serverConfig.ControlConfig.Datastore.Endpoint != "" && serverConfig.ControlConfig.DisableAPIServer {
		return errors.New("invalid flag use; cannot use --disable-apiserver with --datastore-endpoint")
	}

	if serverConfig.ControlConfig.Datastore.Endpoint != "" && serverConfig.ControlConfig.DisableETCD {
		return errors.New("invalid flag use; cannot use --disable-etcd with --datastore-endpoint")
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
		ip, err := util.GetIPFromInterface(cmds.AgentConfig.FlannelIface)
		if err != nil {
			return err
		}
		cmds.AgentConfig.NodeIP.Set(ip)
	}

	if serverConfig.ControlConfig.PrivateIP == "" && len(cmds.AgentConfig.NodeIP) != 0 {
		serverConfig.ControlConfig.PrivateIP = util.GetFirstValidIPString(cmds.AgentConfig.NodeIP)
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

	// if not set, try setting advertise-ip from agent VPN
	if cmds.AgentConfig.VPNAuth != "" {
		vpnInfo, err := vpn.GetVPNInfo(cmds.AgentConfig.VPNAuth)
		if err != nil {
			return err
		}

		// If we are in ipv6-only mode, we should pass the ipv6 address. Otherwise, ipv4
		if utilsnet.IsIPv6(nodeIPs[0]) {
			if vpnInfo.IPv6Address != nil {
				logrus.Infof("Changed advertise-address to %v due to VPN", vpnInfo.IPv6Address)
				if serverConfig.ControlConfig.AdvertiseIP != "" {
					logrus.Warn("Conflict in the config detected. VPN integration overwrites advertise-address but the config is setting the advertise-address parameter")
				}
				serverConfig.ControlConfig.AdvertiseIP = vpnInfo.IPv6Address.String()
			} else {
				return errors.New("tailscale does not provide an ipv6 address")
			}
		} else {
			// We are in dual-stack or ipv4-only mode
			if vpnInfo.IPv4Address != nil {
				logrus.Infof("Changed advertise-address to %v due to VPN", vpnInfo.IPv4Address)
				if serverConfig.ControlConfig.AdvertiseIP != "" {
					logrus.Warn("Conflict in the config detected. VPN integration overwrites advertise-address but the config is setting the advertise-address parameter")
				}
				serverConfig.ControlConfig.AdvertiseIP = vpnInfo.IPv4Address.String()
			} else {
				return errors.New("tailscale does not provide an ipv4 address")
			}
		}
		logrus.Warn("Etcd IP (PrivateIP) remains the local IP. Running etcd traffic over VPN is not recommended due to performance issues")
	} else {

		// if not set, try setting advertise-ip from agent node-external-ip
		if serverConfig.ControlConfig.AdvertiseIP == "" && len(cmds.AgentConfig.NodeExternalIP) != 0 {
			serverConfig.ControlConfig.AdvertiseIP = util.GetFirstValidIPString(cmds.AgentConfig.NodeExternalIP)
		}

		// if not set, try setting advertise-ip from agent node-ip
		if serverConfig.ControlConfig.AdvertiseIP == "" && len(cmds.AgentConfig.NodeIP) != 0 {
			serverConfig.ControlConfig.AdvertiseIP = util.GetFirstValidIPString(cmds.AgentConfig.NodeIP)
		}
	}

	// if we ended up with any advertise-ips, ensure they're added to the SAN list;
	// note that kube-apiserver does not support dual-stack advertise-ip as of 1.21.0:
	/// https://github.com/kubernetes/kubeadm/issues/1612#issuecomment-772583989
	if serverConfig.ControlConfig.AdvertiseIP != "" {
		serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, serverConfig.ControlConfig.AdvertiseIP)
	}

	// configure ClusterIPRanges. Use default 10.42.0.0/16 or fd00:42::/56 if user did not set it
	_, defaultClusterCIDR, defaultServiceCIDR, _ := util.GetDefaultAddresses(nodeIPs[0])
	if len(cmds.ServerConfig.ClusterCIDR) == 0 {
		cmds.ServerConfig.ClusterCIDR.Set(defaultClusterCIDR)
	}
	for _, cidr := range util.SplitStringSlice(cmds.ServerConfig.ClusterCIDR) {
		_, parsed, err := net.ParseCIDR(cidr)
		if err != nil {
			return errors.Wrapf(err, "invalid cluster-cidr %s", cidr)
		}
		serverConfig.ControlConfig.ClusterIPRanges = append(serverConfig.ControlConfig.ClusterIPRanges, parsed)
	}

	// set ClusterIPRange to the first address (first defined IPFamily is preferred)
	serverConfig.ControlConfig.ClusterIPRange = serverConfig.ControlConfig.ClusterIPRanges[0]

	// configure ServiceIPRanges. Use default 10.43.0.0/16 or fd00:43::/112 if user did not set it
	if len(cmds.ServerConfig.ServiceCIDR) == 0 {
		cmds.ServerConfig.ServiceCIDR.Set(defaultServiceCIDR)
	}
	for _, cidr := range util.SplitStringSlice(cmds.ServerConfig.ServiceCIDR) {
		_, parsed, err := net.ParseCIDR(cidr)
		if err != nil {
			return errors.Wrapf(err, "invalid service-cidr %s", cidr)
		}
		serverConfig.ControlConfig.ServiceIPRanges = append(serverConfig.ControlConfig.ServiceIPRanges, parsed)
	}

	// set ServiceIPRange to the first address (first defined IPFamily is preferred)
	serverConfig.ControlConfig.ServiceIPRange = serverConfig.ControlConfig.ServiceIPRanges[0]

	serverConfig.ControlConfig.ServiceNodePortRange, err = utilnet.ParsePortRange(cfg.ServiceNodePortRange)
	if err != nil {
		return errors.Wrapf(err, "invalid port range %s", cfg.ServiceNodePortRange)
	}

	// the apiserver service does not yet support dual-stack operation
	_, apiServerServiceIP, err := options.ServiceIPRange(*serverConfig.ControlConfig.ServiceIPRanges[0])
	if err != nil {
		return err
	}
	serverConfig.ControlConfig.SANs = append(serverConfig.ControlConfig.SANs, apiServerServiceIP.String())

	// If cluster-dns CLI arg is not set, we set ClusterDNS address to be the first IPv4 ServiceCIDR network + 10,
	// i.e. when you set service-cidr to 192.168.0.0/16 and don't provide cluster-dns, it will be set to 192.168.0.10
	// If there are no IPv4 ServiceCIDRs, an IPv6 ServiceCIDRs will be used.
	// If neither of IPv4 or IPv6 are found an error is raised.
	if len(cmds.ServerConfig.ClusterDNS) == 0 {
		for _, svcCIDR := range serverConfig.ControlConfig.ServiceIPRanges {
			clusterDNS, err := utilsnet.GetIndexedIP(svcCIDR, 10)
			if err != nil {
				return errors.Wrap(err, "cannot configure default cluster-dns address")
			}
			serverConfig.ControlConfig.ClusterDNSs = append(serverConfig.ControlConfig.ClusterDNSs, clusterDNS)
		}
	} else {
		for _, ip := range util.SplitStringSlice(cmds.ServerConfig.ClusterDNS) {
			parsed := net.ParseIP(ip)
			if parsed == nil {
				return fmt.Errorf("invalid cluster-dns address %s", ip)
			}
			serverConfig.ControlConfig.ClusterDNSs = append(serverConfig.ControlConfig.ClusterDNSs, parsed)
		}
	}

	serverConfig.ControlConfig.ClusterDNS = serverConfig.ControlConfig.ClusterDNSs[0]

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
	serverConfig.ControlConfig.Disables = map[string]bool{}
	for _, disable := range util.SplitStringSlice(app.StringSlice("disable")) {
		disable = strings.TrimSpace(disable)
		serverConfig.ControlConfig.Skips[disable] = true
		serverConfig.ControlConfig.Disables[disable] = true
	}
	if serverConfig.ControlConfig.Skips["servicelb"] {
		serverConfig.ControlConfig.DisableServiceLB = true
	}

	if serverConfig.ControlConfig.DisableCCM && serverConfig.ControlConfig.DisableServiceLB {
		serverConfig.ControlConfig.Skips["ccm"] = true
		serverConfig.ControlConfig.Disables["ccm"] = true
	}

	tlsMinVersionArg := getArgValueFromList("tls-min-version", serverConfig.ControlConfig.ExtraAPIArgs)
	serverConfig.ControlConfig.MinTLSVersion = tlsMinVersionArg
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
	tlsCipherSuitesArg := getArgValueFromList("tls-cipher-suites", serverConfig.ControlConfig.ExtraAPIArgs)
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
		serverConfig.ControlConfig.ExtraAPIArgs = append(serverConfig.ControlConfig.ExtraAPIArgs, "tls-cipher-suites="+strings.Join(tlsCipherSuites, ","))
	}
	serverConfig.ControlConfig.CipherSuites = tlsCipherSuites
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
		serverConfig.ControlConfig.DisableServiceLB = true

		// If the supervisor and apiserver are on the same port, everything is running embedded
		// and we don't need the kubelet or containerd up to perform a cluster reset.
		if serverConfig.ControlConfig.SupervisorPort == serverConfig.ControlConfig.HTTPSPort {
			cfg.DisableAgent = true
		}

		// If the user uses the cluster-reset argument in a cluster that has a ServerURL, we must return an error
		// to remove the server flag on the configuration or in the cli
		if serverConfig.ControlConfig.JoinURL != "" {
			return errors.New("cannot perform cluster-reset while server URL is set - remove server from configuration before resetting")
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

	go cmds.WriteCoverage(ctx)

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

	url := fmt.Sprintf("https://%s:%d", serverConfig.ControlConfig.BindAddressOrLoopback(false, true), serverConfig.ControlConfig.SupervisorPort)
	token, err := clientaccess.FormatToken(serverConfig.ControlConfig.Runtime.AgentToken, serverConfig.ControlConfig.Runtime.ServerCA)
	if err != nil {
		return err
	}

	agentConfig := cmds.AgentConfig
	agentConfig.ContainerRuntimeReady = containerRuntimeReady
	agentConfig.Debug = app.GlobalBool("debug")
	agentConfig.DataDir = filepath.Dir(serverConfig.ControlConfig.DataDir)
	agentConfig.ServerURL = url
	agentConfig.Token = token
	agentConfig.DisableLoadBalancer = !serverConfig.ControlConfig.DisableAPIServer
	agentConfig.DisableServiceLB = serverConfig.ControlConfig.DisableServiceLB
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

	// Until the agent is run and retrieves config from the server, we won't know
	// if the embedded registry is enabled. If it is not enabled, these are not
	// used as the registry is never started.
	registry := spegel.DefaultRegistry
	registry.Bootstrapper = spegel.NewChainingBootstrapper(
		spegel.NewServerBootstrapper(&serverConfig.ControlConfig),
		spegel.NewAgentBootstrapper(cfg.ServerURL, token, agentConfig.DataDir),
		spegel.NewSelfBootstrapper(),
	)
	registry.Router = func(ctx context.Context, nodeConfig *config.Node) (*mux.Router, error) {
		return https.Start(ctx, nodeConfig, serverConfig.ControlConfig.Runtime)
	}

	// same deal for metrics - these are not used if the extra metrics listener is not enabled.
	metrics := k3smetrics.DefaultMetrics
	metrics.Router = func(ctx context.Context, nodeConfig *config.Node) (*mux.Router, error) {
		return https.Start(ctx, nodeConfig, serverConfig.ControlConfig.Runtime)
	}

	// and for pprof as well
	pprof := profile.DefaultProfiler
	pprof.Router = func(ctx context.Context, nodeConfig *config.Node) (*mux.Router, error) {
		return https.Start(ctx, nodeConfig, serverConfig.ControlConfig.Runtime)
	}

	if cfg.DisableAgent {
		agentConfig.ContainerRuntimeEndpoint = "/dev/null"
		return agent.RunStandalone(ctx, agentConfig)
	}

	return agent.Run(ctx, agentConfig)
}

// validateNetworkConfig ensures that the network configuration values make sense.
func validateNetworkConfiguration(serverConfig server.Config) error {
	switch serverConfig.ControlConfig.EgressSelectorMode {
	case config.EgressSelectorModeCluster, config.EgressSelectorModePod:
	case config.EgressSelectorModeAgent, config.EgressSelectorModeDisabled:
		if serverConfig.DisableAgent {
			logrus.Warn("Webhooks and apiserver aggregation may not function properly without an agent; please set egress-selector-mode to 'cluster' or 'pod'")
		}
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
