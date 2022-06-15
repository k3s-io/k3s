package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	utilsnet "k8s.io/utils/net"
)

const (
	FlannelBackendNone            = "none"
	FlannelBackendVXLAN           = "vxlan"
	FlannelBackendHostGW          = "host-gw"
	FlannelBackendIPSEC           = "ipsec"
	FlannelBackendWireguard       = "wireguard"
	FlannelBackendWireguardNative = "wireguard-native"
	EgressSelectorModeAgent       = "agent"
	EgressSelectorModeCluster     = "cluster"
	EgressSelectorModeDisabled    = "disabled"
	EgressSelectorModePod         = "pod"
	CertificateRenewDays          = 90
	StreamServerPort              = "10010"
	KubeletPort                   = "10250"
)

// These ports can always be accessed via the tunnel server, at the loopback address.
// Other addresses and ports are only accessible via the tunnel on newer agents, when used by a pod.
var KubeletReservedPorts = map[string]bool{
	StreamServerPort: true,
	KubeletPort:      true,
}

type Node struct {
	ContainerRuntimeEndpoint string
	NoFlannel                bool
	SELinux                  bool
	FlannelBackend           string
	FlannelConfFile          string
	FlannelConfOverride      bool
	FlannelIface             *net.Interface
	FlannelIPv6Masq          bool
	EgressSelectorMode       string
	Containerd               Containerd
	Images                   string
	AgentConfig              Agent
	Token                    string
	Certificate              *tls.Certificate
	ServerHTTPSPort          int
}

type Containerd struct {
	Address  string
	Log      string
	Root     string
	State    string
	Config   string
	Opt      string
	Template string
	SELinux  bool
}

type Agent struct {
	PodManifests            string
	NodeName                string
	NodeConfigPath          string
	ServingKubeletCert      string
	ServingKubeletKey       string
	ServiceCIDR             *net.IPNet
	ServiceCIDRs            []*net.IPNet
	ServiceNodePortRange    utilnet.PortRange
	ClusterCIDR             *net.IPNet
	ClusterCIDRs            []*net.IPNet
	ClusterDNS              net.IP
	ClusterDNSs             []net.IP
	ClusterDomain           string
	ResolvConf              string
	RootDir                 string
	KubeConfigKubelet       string
	KubeConfigKubeProxy     string
	KubeConfigK3sController string
	NodeIP                  string
	NodeIPs                 []net.IP
	NodeExternalIP          string
	NodeExternalIPs         []net.IP
	RuntimeSocket           string
	ImageServiceSocket      string
	ListenAddress           string
	ClientCA                string
	CNIBinDir               string
	CNIConfDir              string
	ExtraKubeletArgs        []string
	ExtraKubeProxyArgs      []string
	PauseImage              string
	Snapshotter             string
	Systemd                 bool
	CNIPlugin               bool
	NodeTaints              []string
	NodeLabels              []string
	ImageCredProvBinDir     string
	ImageCredProvConfig     string
	IPSECPSK                string
	FlannelCniConfFile      string
	StrongSwanDir           string
	PrivateRegistry         string
	SystemDefaultRegistry   string
	AirgapExtraRegistry     []string
	DisableCCM              bool
	DisableNPC              bool
	Rootless                bool
	ProtectKernelDefaults   bool
	DisableServiceLB        bool
	EnableIPv4              bool
	EnableIPv6              bool
}

// CriticalControlArgs contains parameters that all control plane nodes in HA must share
type CriticalControlArgs struct {
	ClusterDNSs           []net.IP
	ClusterIPRanges       []*net.IPNet
	ClusterDNS            net.IP
	ClusterDomain         string
	ClusterIPRange        *net.IPNet
	DisableCCM            bool
	DisableHelmController bool
	DisableNPC            bool
	DisableServiceLB      bool
	FlannelBackend        string
	FlannelIPv6Masq       bool
	EgressSelectorMode    string
	NoCoreDNS             bool
	ServiceIPRange        *net.IPNet
	ServiceIPRanges       []*net.IPNet
}

type Control struct {
	CriticalControlArgs
	AdvertisePort int
	AdvertiseIP   string
	// The port which kubectl clients can access k8s
	HTTPSPort int
	// The port which custom k3s API runs on
	SupervisorPort int
	// The port which kube-apiserver runs on
	APIServerPort            int
	APIServerBindAddress     string
	AgentToken               string `json:"-"`
	Token                    string `json:"-"`
	ServiceNodePortRange     *utilnet.PortRange
	KubeConfigOutput         string
	KubeConfigMode           string
	DataDir                  string
	Datastore                endpoint.Config `json:"-"`
	Disables                 map[string]bool
	DisableAPIServer         bool
	DisableControllerManager bool
	DisableETCD              bool
	DisableKubeProxy         bool
	DisableScheduler         bool
	EnablePProf              bool
	ExtraAPIArgs             []string
	ExtraControllerArgs      []string
	ExtraCloudControllerArgs []string
	ExtraEtcdArgs            []string
	ExtraSchedulerAPIArgs    []string
	NoLeaderElect            bool
	JoinURL                  string
	IPSECPSK                 string
	DefaultLocalStoragePath  string
	Skips                    map[string]bool
	SystemDefaultRegistry    string
	ClusterInit              bool
	ClusterReset             bool
	ClusterResetRestorePath  string
	EncryptSecrets           bool
	EncryptForce             bool
	EncryptSkip              bool
	TLSMinVersion            uint16
	TLSCipherSuites          []uint16
	EtcdSnapshotName         string        `json:"-"`
	EtcdDisableSnapshots     bool          `json:"-"`
	EtcdExposeMetrics        bool          `json:"-"`
	EtcdSnapshotDir          string        `json:"-"`
	EtcdSnapshotCron         string        `json:"-"`
	EtcdSnapshotRetention    int           `json:"-"`
	EtcdSnapshotCompress     bool          `json:"-"`
	EtcdListFormat           string        `json:"-"`
	EtcdS3                   bool          `json:"-"`
	EtcdS3Endpoint           string        `json:"-"`
	EtcdS3EndpointCA         string        `json:"-"`
	EtcdS3SkipSSLVerify      bool          `json:"-"`
	EtcdS3AccessKey          string        `json:"-"`
	EtcdS3SecretKey          string        `json:"-"`
	EtcdS3BucketName         string        `json:"-"`
	EtcdS3Region             string        `json:"-"`
	EtcdS3Folder             string        `json:"-"`
	EtcdS3Timeout            time.Duration `json:"-"`
	EtcdS3Insecure           bool          `json:"-"`
	ServerNodeName           string

	BindAddress string
	SANs        []string
	PrivateIP   string
	Runtime     *ControlRuntime `json:"-"`
}

// BindAddressOrLoopback returns an IPv4 or IPv6 address suitable for embedding in server
// URLs. If a bind address was configured, that is returned. If the chooseHostInterface
// parameter is true, and a suitable default interface can be found, that interface's
// address is returned.  If neither of the previous were used, the loopback address is
// returned. IPv6 addresses are enclosed in square brackets, as per RFC2732.
func (c *Control) BindAddressOrLoopback(chooseHostInterface bool) string {
	ip := c.BindAddress
	if ip == "" && chooseHostInterface {
		if hostIP, _ := utilnet.ChooseHostInterface(); len(hostIP) > 0 {
			ip = hostIP.String()
		}
	}
	if utilsnet.IsIPv6String(ip) {
		return fmt.Sprintf("[%s]", ip)
	} else if ip != "" {
		return ip
	}
	return c.Loopback()
}

// Loopback returns an IPv4 or IPv6 loopback address, depending on whether the cluster
// service CIDRs indicate an IPv4/Dual-Stack or IPv6 only cluster.  IPv6 addresses are
// enclosed in square brackets, as per RFC2732.
func (c *Control) Loopback() string {
	if IPv6OnlyService, _ := util.IsIPv6OnlyCIDRs(c.ServiceIPRanges); IPv6OnlyService {
		return "[::1]"
	}
	return "127.0.0.1"
}

type ControlRuntimeBootstrap struct {
	ETCDServerCA       string
	ETCDServerCAKey    string
	ETCDPeerCA         string
	ETCDPeerCAKey      string
	ServerCA           string
	ServerCAKey        string
	ClientCA           string
	ClientCAKey        string
	ServiceKey         string
	PasswdFile         string
	RequestHeaderCA    string
	RequestHeaderCAKey string
	IPSECKey           string
	EncryptionConfig   string
	EncryptionHash     string
}

type ControlRuntime struct {
	ControlRuntimeBootstrap

	HTTPBootstrap                       bool
	APIServerReady                      <-chan struct{}
	AgentReady                          <-chan struct{}
	ETCDReady                           <-chan struct{}
	StartupHooksWg                      *sync.WaitGroup
	ClusterControllerStart              func(ctx context.Context) error
	LeaderElectedClusterControllerStart func(ctx context.Context) error

	ClientKubeAPICert string
	ClientKubeAPIKey  string
	NodePasswdFile    string

	KubeConfigAdmin           string
	KubeConfigController      string
	KubeConfigScheduler       string
	KubeConfigAPIServer       string
	KubeConfigCloudController string

	ServingKubeAPICert string
	ServingKubeAPIKey  string
	ServingKubeletKey  string
	ServerToken        string
	AgentToken         string
	APIServer          http.Handler
	Handler            http.Handler
	Tunnel             http.Handler
	Authenticator      authenticator.Request

	EgressSelectorConfig string

	ClientAuthProxyCert string
	ClientAuthProxyKey  string

	ClientAdminCert           string
	ClientAdminKey            string
	ClientControllerCert      string
	ClientControllerKey       string
	ClientSchedulerCert       string
	ClientSchedulerKey        string
	ClientKubeProxyCert       string
	ClientKubeProxyKey        string
	ClientKubeletKey          string
	ClientCloudControllerCert string
	ClientCloudControllerKey  string
	ClientK3sControllerCert   string
	ClientK3sControllerKey    string

	ServerETCDCert           string
	ServerETCDKey            string
	PeerServerClientETCDCert string
	PeerServerClientETCDKey  string
	ClientETCDCert           string
	ClientETCDKey            string

	Core       *core.Factory
	EtcdConfig endpoint.ETCDConfig
}

type ArgString []string

func (a ArgString) String() string {
	b := strings.Builder{}
	for _, s := range a {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(s)
	}
	return b.String()
}

// GetArgs appends extra arguments to existing arguments with logic to override any default
// arguments whilst also allowing to prefix and suffix default string slice arguments.
func GetArgs(initialArgs map[string]string, extraArgs []string) []string {
	const hyphens = "--"

	multiArgs := make(map[string][]string)

	for _, unsplitArg := range extraArgs {
		splitArg := strings.SplitN(strings.TrimPrefix(unsplitArg, hyphens), "=", 2)
		arg := splitArg[0]
		value := "true"
		if len(splitArg) > 1 {
			value = splitArg[1]
		}

		// After the first iteration, initial args will be empty when handling
		// duplicate arguments as they will form part of existingValues
		cleanedArg := strings.TrimRight(arg, "-+")
		initialValue, initialValueExists := initialArgs[cleanedArg]
		existingValues, existingValuesFound := multiArgs[cleanedArg]

		newValues := make([]string, 0)
		if strings.HasSuffix(arg, "+") { // Append value to initial args
			if initialValueExists {
				newValues = append(newValues, initialValue)
			}
			if existingValuesFound {
				newValues = append(newValues, existingValues...)
			}
			newValues = append(newValues, value)

		} else if strings.HasSuffix(arg, "-") { // Prepend value to initial args
			newValues = append(newValues, value)
			if initialValueExists {
				newValues = append(newValues, initialValue)
			}
			if existingValuesFound {
				newValues = append(newValues, existingValues...)
			}
		} else { // Append value ignoring initial args
			if existingValuesFound {
				newValues = append(newValues, existingValues...)
			}
			newValues = append(newValues, value)
		}

		delete(initialArgs, cleanedArg)
		multiArgs[cleanedArg] = newValues

	}

	// Add any remaining initial args to the map
	for arg, value := range initialArgs {
		multiArgs[arg] = []string{value}
	}

	// Get args so we can output them sorted whilst preserving the order of
	// repeated keys
	var keys []string
	for arg := range multiArgs {
		keys = append(keys, arg)
	}
	sort.Strings(keys)

	var args []string
	for _, arg := range keys {
		values := multiArgs[arg]
		for _, value := range values {
			cmd := fmt.Sprintf("%s%s=%s", hyphens, strings.TrimPrefix(arg, hyphens), value)
			args = append(args, cmd)
		}
	}

	return args
}
