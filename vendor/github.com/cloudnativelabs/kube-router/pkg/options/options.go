package options

import (
	"net"
	"strconv"
	"time"

	"github.com/spf13/pflag"
)

const DefaultBgpPort = 179
const DefaultBgpHoldTime = 90 * time.Second

type KubeRouterConfig struct {
	AdvertiseClusterIP             bool
	AdvertiseExternalIP            bool
	AdvertiseLoadBalancerIP        bool
	AdvertiseNodePodCidr           bool
	AutoMTU                        bool
	BGPGracefulRestart             bool
	BGPGracefulRestartDeferralTime time.Duration
	BGPGracefulRestartTime         time.Duration
	BGPHoldTime                    time.Duration
	BGPPort                        uint32
	CacheSyncTimeout               time.Duration
	CleanupConfig                  bool
	ClusterAsn                     uint
	ClusterIPCIDR                  string
	DisableSrcDstCheck             bool
	EnableCNI                      bool
	EnableiBGP                     bool
	EnableOverlay                  bool
	EnablePodEgress                bool
	EnablePprof                    bool
	ExcludedCidrs                  []string
	ExternalIPCIDRs                []string
	FullMeshMode                   bool
	GlobalHairpinMode              bool
	HealthPort                     uint16
	HelpRequested                  bool
	HostnameOverride               string
	IPTablesSyncPeriod             time.Duration
	IpvsGracefulPeriod             time.Duration
	IpvsGracefulTermination        bool
	IpvsPermitAll                  bool
	IpvsSyncPeriod                 time.Duration
	Kubeconfig                     string
	MasqueradeAll                  bool
	Master                         string
	MetricsEnabled                 bool
	MetricsPath                    string
	MetricsPort                    uint16
	NodePortBindOnAllIP            bool
	NodePortRange                  string
	OverlayType                    string
	OverrideNextHop                bool
	PeerASNs                       []uint
	PeerMultihopTTL                uint8
	PeerPasswords                  []string
	PeerPasswordsFile              string
	PeerPorts                      []uint
	PeerRouters                    []net.IP
	RouterID                       string
	RoutesSyncPeriod               time.Duration
	RunFirewall                    bool
	RunRouter                      bool
	RunServiceProxy                bool
	RuntimeEndpoint                string
	Version                        bool
	VLevel                         string
	// FullMeshPassword    string
}

func NewKubeRouterConfig() *KubeRouterConfig {
	return &KubeRouterConfig{
		BGPGracefulRestartDeferralTime: 360 * time.Second,
		BGPGracefulRestartTime:         90 * time.Second,
		BGPHoldTime:                    90 * time.Second,
		CacheSyncTimeout:               1 * time.Minute,
		ClusterIPCIDR:                  "10.96.0.0/12",
		EnableOverlay:                  true,
		IPTablesSyncPeriod:             5 * time.Minute,
		IpvsGracefulPeriod:             30 * time.Second,
		IpvsSyncPeriod:                 5 * time.Minute,
		NodePortRange:                  "30000-32767",
		OverlayType:                    "subnet",
		RoutesSyncPeriod:               5 * time.Minute,
	}
}

func (s *KubeRouterConfig) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&s.AdvertiseClusterIP, "advertise-cluster-ip", false,
		"Add Cluster IP of the service to the RIB so that it gets advertises to the BGP peers.")
	fs.BoolVar(&s.AdvertiseExternalIP, "advertise-external-ip", false,
		"Add External IP of service to the RIB so that it gets advertised to the BGP peers.")
	fs.BoolVar(&s.AdvertiseLoadBalancerIP, "advertise-loadbalancer-ip", false,
		"Add LoadbBalancer IP of service status as set by the LB provider to the RIB so that it gets advertised to the BGP peers.")
	fs.BoolVar(&s.AdvertiseNodePodCidr, "advertise-pod-cidr", true,
		"Add Node's POD cidr to the RIB so that it gets advertised to the BGP peers.")
	fs.BoolVar(&s.AutoMTU, "auto-mtu", true,
		"Auto detect and set the largest possible MTU for pod interfaces.")
	fs.BoolVar(&s.BGPGracefulRestart, "bgp-graceful-restart", false,
		"Enables the BGP Graceful Restart capability so that routes are preserved on unexpected restarts")
	fs.DurationVar(&s.BGPGracefulRestartDeferralTime, "bgp-graceful-restart-deferral-time", s.BGPGracefulRestartDeferralTime,
		"BGP Graceful restart deferral time according to RFC4724 4.1, maximum 18h.")
	fs.DurationVar(&s.BGPGracefulRestartTime, "bgp-graceful-restart-time", s.BGPGracefulRestartTime,
		"BGP Graceful restart time according to RFC4724 3, maximum 4095s.")
	fs.DurationVar(&s.BGPHoldTime, "bgp-holdtime", DefaultBgpHoldTime,
		"This parameter is mainly used to modify the holdtime declared to BGP peer. When Kube-router goes down abnormally, the local saving time of BGP route will be affected.Holdtime must be in the range 3s to 18h12m16s.")
	fs.Uint32Var(&s.BGPPort, "bgp-port", DefaultBgpPort,
		"The port open for incoming BGP connections and to use for connecting with other BGP peers.")
	fs.DurationVar(&s.CacheSyncTimeout, "cache-sync-timeout", s.CacheSyncTimeout,
		"The timeout for cache synchronization (e.g. '5s', '1m'). Must be greater than 0.")
	fs.BoolVar(&s.CleanupConfig, "cleanup-config", false,
		"Cleanup iptables rules, ipvs, ipset configuration and exit.")
	fs.UintVar(&s.ClusterAsn, "cluster-asn", s.ClusterAsn,
		"ASN number under which cluster nodes will run iBGP.")
	fs.BoolVar(&s.DisableSrcDstCheck, "disable-source-dest-check", true,
		"Disable the source-dest-check attribute for AWS EC2 instances. When this option is false, it must be set some other way.")
	fs.BoolVar(&s.EnableCNI, "enable-cni", true,
		"Enable CNI plugin. Disable if you want to use kube-router features alongside another CNI plugin.")
	fs.BoolVar(&s.EnableiBGP, "enable-ibgp", true,
		"Enables peering with nodes with the same ASN, if disabled will only peer with external BGP peers")
	fs.BoolVar(&s.EnableOverlay, "enable-overlay", true,
		"When enable-overlay is set to true, IP-in-IP tunneling is used for pod-to-pod networking across nodes in different subnets. "+
			"When set to false no tunneling is used and routing infrastructure is expected to route traffic for pod-to-pod networking across nodes in different subnets")
	fs.BoolVar(&s.EnablePodEgress, "enable-pod-egress", true,
		"SNAT traffic from Pods to destinations outside the cluster.")
	fs.BoolVar(&s.EnablePprof, "enable-pprof", false,
		"Enables pprof for debugging performance and memory leak issues.")
	fs.StringSliceVar(&s.ExcludedCidrs, "excluded-cidrs", s.ExcludedCidrs,
		"Excluded CIDRs are used to exclude IPVS rules from deletion.")
	fs.BoolVar(&s.GlobalHairpinMode, "hairpin-mode", false,
		"Add iptables rules for every Service Endpoint to support hairpin traffic.")
	fs.Uint16Var(&s.HealthPort, "health-port", 20244, "Health check port, 0 = Disabled")
	fs.BoolVarP(&s.HelpRequested, "help", "h", false,
		"Print usage information.")
	fs.StringVar(&s.HostnameOverride, "hostname-override", s.HostnameOverride,
		"Overrides the NodeName of the node. Set this if kube-router is unable to determine your NodeName automatically.")
	fs.DurationVar(&s.IPTablesSyncPeriod, "iptables-sync-period", s.IPTablesSyncPeriod,
		"The delay between iptables rule synchronizations (e.g. '5s', '1m'). Must be greater than 0.")
	fs.DurationVar(&s.IpvsGracefulPeriod, "ipvs-graceful-period", s.IpvsGracefulPeriod,
		"The graceful period before removing destinations from IPVS services (e.g. '5s', '1m', '2h22m'). Must be greater than 0.")
	fs.BoolVar(&s.IpvsGracefulTermination, "ipvs-graceful-termination", false,
		"Enables the experimental IPVS graceful terminaton capability")
	fs.BoolVar(&s.IpvsPermitAll, "ipvs-permit-all", true,
		"Enables rule to accept all incoming traffic to service VIP's on the node.")
	fs.DurationVar(&s.IpvsSyncPeriod, "ipvs-sync-period", s.IpvsSyncPeriod,
		"The delay between ipvs config synchronizations (e.g. '5s', '1m', '2h22m'). Must be greater than 0.")
	fs.StringVar(&s.Kubeconfig, "kubeconfig", s.Kubeconfig,
		"Path to kubeconfig file with authorization information (the master location is set by the master flag).")
	fs.BoolVar(&s.MasqueradeAll, "masquerade-all", false,
		"SNAT all traffic to cluster IP/node port.")
	fs.StringVar(&s.Master, "master", s.Master,
		"The address of the Kubernetes API server (overrides any value in kubeconfig).")
	fs.StringVar(&s.MetricsPath, "metrics-path", "/metrics", "Prometheus metrics path")
	fs.Uint16Var(&s.MetricsPort, "metrics-port", 0, "Prometheus metrics port, (Default 0, Disabled)")
	fs.BoolVar(&s.NodePortBindOnAllIP, "nodeport-bindon-all-ip", false,
		"For service of NodePort type create IPVS service that listens on all IP's of the node.")
	fs.BoolVar(&s.FullMeshMode, "nodes-full-mesh", true,
		"Each node in the cluster will setup BGP peering with rest of the nodes.")
	fs.StringVar(&s.OverlayType, "overlay-type", s.OverlayType,
		"Possible values: subnet,full - "+
			"When set to \"subnet\", the default, default \"--enable-overlay=true\" behavior is used. "+
			"When set to \"full\", it changes \"--enable-overlay=true\" default behavior so that IP-in-IP tunneling is used for pod-to-pod networking across nodes regardless of the subnet the nodes are in.")
	fs.BoolVar(&s.OverrideNextHop, "override-nexthop", false, "Override the next-hop in bgp routes sent to peers with the local ip.")
	fs.UintSliceVar(&s.PeerASNs, "peer-router-asns", s.PeerASNs,
		"ASN numbers of the BGP peer to which cluster nodes will advertise cluster ip and node's pod cidr.")
	fs.IPSliceVar(&s.PeerRouters, "peer-router-ips", s.PeerRouters,
		"The ip address of the external router to which all nodes will peer and advertise the cluster ip and pod cidr's.")
	fs.Uint8Var(&s.PeerMultihopTTL, "peer-router-multihop-ttl", s.PeerMultihopTTL,
		"Enable eBGP multihop supports -- sets multihop-ttl. (Relevant only if ttl >= 2)")
	fs.StringSliceVar(&s.PeerPasswords, "peer-router-passwords", s.PeerPasswords,
		"Password for authenticating against the BGP peer defined with \"--peer-router-ips\".")
	fs.StringVar(&s.PeerPasswordsFile, "peer-router-passwords-file", s.PeerPasswordsFile,
		"Path to file containing password for authenticating against the BGP peer defined with \"--peer-router-ips\". --peer-router-passwords will be preferred if both are set.")
	fs.UintSliceVar(&s.PeerPorts, "peer-router-ports", s.PeerPorts,
		"The remote port of the external BGP to which all nodes will peer. If not set, default BGP port ("+strconv.Itoa(DefaultBgpPort)+") will be used.")
	fs.StringVar(&s.RouterID, "router-id", "", "BGP router-id. Must be specified in a ipv6 only cluster.")
	fs.DurationVar(&s.RoutesSyncPeriod, "routes-sync-period", s.RoutesSyncPeriod,
		"The delay between route updates and advertisements (e.g. '5s', '1m', '2h22m'). Must be greater than 0.")
	fs.BoolVar(&s.RunFirewall, "run-firewall", true,
		"Enables Network Policy -- sets up iptables to provide ingress firewall for pods.")
	fs.BoolVar(&s.RunRouter, "run-router", true,
		"Enables Pod Networking -- Advertises and learns the routes to Pods via iBGP.")
	fs.BoolVar(&s.RunServiceProxy, "run-service-proxy", true,
		"Enables Service Proxy -- sets up IPVS for Kubernetes Services.")
	fs.StringVar(&s.RuntimeEndpoint, "runtime-endpoint", "",
		"Path to CRI compatible container runtime socket (used for DSR mode). Currently known working with containerd.")
	fs.StringVar(&s.ClusterIPCIDR, "service-cluster-ip-range", s.ClusterIPCIDR,
		"CIDR value from which service cluster IPs are assigned. Default: 10.96.0.0/12")
	fs.StringSliceVar(&s.ExternalIPCIDRs, "service-external-ip-range", s.ExternalIPCIDRs,
		"Specify external IP CIDRs that are used for inter-cluster communication (can be specified multiple times)")
	fs.StringVar(&s.NodePortRange, "service-node-port-range", s.NodePortRange,
		"NodePort range specified with either a hyphen or colon")
	fs.StringVarP(&s.VLevel, "v", "v", "0", "log level for V logs")
	fs.BoolVarP(&s.Version, "version", "V", false,
		"Print version information.")
}
