package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/k3s-io/k3s/pkg/generated/controllers/k3s.cattle.io"
	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/leader"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/client-go/tools/record"
	utilsnet "k8s.io/utils/net"
)

const (
	FlannelBackendNone            = "none"
	FlannelBackendVXLAN           = "vxlan"
	FlannelBackendHostGW          = "host-gw"
	FlannelBackendWireguardNative = "wireguard-native"
	FlannelBackendTailscale       = "tailscale"
	EgressSelectorModeAgent       = "agent"
	EgressSelectorModeCluster     = "cluster"
	EgressSelectorModeDisabled    = "disabled"
	EgressSelectorModePod         = "pod"
	CertificateRenewDays          = 90
	StreamServerPort              = "10010"
)

type Node struct {
	Docker                   bool
	ContainerRuntimeEndpoint string
	ImageServiceEndpoint     string
	NoFlannel                bool
	SELinux                  bool
	EnablePProf              bool
	SupervisorMetrics        bool
	EmbeddedRegistry         bool
	FlannelBackend           string
	FlannelConfFile          string
	FlannelConfOverride      bool
	FlannelIface             *net.Interface
	FlannelIPv6Masq          bool
	FlannelExternalIP        bool
	EgressSelectorMode       string
	Containerd               Containerd
	CRIDockerd               CRIDockerd
	Images                   string
	AgentConfig              Agent
	Token                    string
	Certificate              *tls.Certificate
	ServerHTTPSPort          int
	SupervisorPort           int
	DefaultRuntime           string
}

type EtcdS3 struct {
	AccessKey     string          `json:"accessKey,omitempty"`
	Bucket        string          `json:"bucket,omitempty"`
	ConfigSecret  string          `json:"configSecret,omitempty"`
	Endpoint      string          `json:"endpoint,omitempty"`
	EndpointCA    string          `json:"endpointCA,omitempty"`
	Folder        string          `json:"folder,omitempty"`
	Proxy         string          `json:"proxy,omitempty"`
	Region        string          `json:"region,omitempty"`
	SecretKey     string          `json:"secretKey,omitempty"`
	Insecure      bool            `json:"insecure,omitempty"`
	SkipSSLVerify bool            `json:"skipSSLVerify,omitempty"`
	Timeout       metav1.Duration `json:"timeout,omitempty"`
}

type Containerd struct {
	Address       string
	Log           string
	Root          string
	State         string
	Config        string
	Opt           string
	Template      string
	BlockIOConfig string
	RDTConfig     string
	Registry      string
	NoDefault     bool
	SELinux       bool
	Debug         bool
}

type CRIDockerd struct {
	Address string
	Root    string
}

type Agent struct {
	PodManifests            string
	NodeName                string
	NodeConfigPath          string
	ClientKubeletCert       string
	ClientKubeletKey        string
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
	NodeInternalDNSs        []string
	NodeExternalDNSs        []string
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
	Registry                *registries.Registry
	SystemDefaultRegistry   string
	AirgapExtraRegistry     []string
	DisableCCM              bool
	DisableNPC              bool
	MinTLSVersion           string
	CipherSuites            []string
	Rootless                bool
	ProtectKernelDefaults   bool
	DisableServiceLB        bool
	EnableIPv4              bool
	EnableIPv6              bool
	VLevel                  int
	VModule                 string
	LogFile                 string
	AlsoLogToStderr         bool
}

// CriticalControlArgs contains parameters that all control plane nodes in HA must share
// The cli tag is used to provide better error information to the user on mismatch
type CriticalControlArgs struct {
	ClusterDNSs           []net.IP     `cli:"cluster-dns"`
	ClusterIPRanges       []*net.IPNet `cli:"cluster-cidr"`
	ClusterDNS            net.IP       `cli:"cluster-dns"`
	ClusterDomain         string       `cli:"cluster-domain"`
	ClusterIPRange        *net.IPNet   `cli:"cluster-cidr"`
	DisableCCM            bool         `cli:"disable-cloud-controller"`
	DisableHelmController bool         `cli:"disable-helm-controller"`
	DisableNPC            bool         `cli:"disable-network-policy"`
	DisableServiceLB      bool         `cli:"disable-service-lb"`
	EncryptSecrets        bool         `cli:"secrets-encryption"`
	EmbeddedRegistry      bool         `cli:"embedded-registry"`
	FlannelBackend        string       `cli:"flannel-backend"`
	FlannelIPv6Masq       bool         `cli:"flannel-ipv6-masq"`
	FlannelExternalIP     bool         `cli:"flannel-external-ip"`
	EgressSelectorMode    string       `cli:"egress-selector-mode"`
	ServiceIPRange        *net.IPNet   `cli:"service-cidr"`
	ServiceIPRanges       []*net.IPNet `cli:"service-cidr"`
	SupervisorMetrics     bool         `cli:"supervisor-metrics"`
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
	KubeConfigGroup          string
	HelmJobImage             string
	DataDir                  string
	KineTLS                  bool
	Datastore                endpoint.Config `json:"-"`
	Disables                 map[string]bool
	DisableAgent             bool
	DisableAPIServer         bool
	DisableControllerManager bool
	DisableETCD              bool
	DisableKubeProxy         bool
	DisableScheduler         bool
	DisableServiceLB         bool
	Rootless                 bool
	ServiceLBNamespace       string
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
	MinTLSVersion            string
	CipherSuites             []string
	TLSMinVersion            uint16   `json:"-"`
	TLSCipherSuites          []uint16 `json:"-"`
	EtcdSnapshotName         string   `json:"-"`
	EtcdDisableSnapshots     bool     `json:"-"`
	EtcdExposeMetrics        bool     `json:"-"`
	EtcdSnapshotDir          string   `json:"-"`
	EtcdSnapshotCron         string   `json:"-"`
	EtcdSnapshotRetention    int      `json:"-"`
	EtcdSnapshotCompress     bool     `json:"-"`
	EtcdListFormat           string   `json:"-"`
	EtcdS3                   *EtcdS3  `json:"-"`
	ServerNodeName           string
	VLevel                   int
	VModule                  string

	BindAddress string
	SANs        []string
	SANSecurity bool
	PrivateIP   string
	Runtime     *ControlRuntime `json:"-"`
}

// BindAddressOrLoopback returns an IPv4 or IPv6 address suitable for embedding in
// server URLs. If a bind address was configured, that is returned. If the
// chooseHostInterface parameter is true, and a suitable default interface can be
// found, that interface's address is returned.  If neither of the previous were used,
// the loopback address is returned. If the urlSafe parameter is true, IPv6 addresses
// are enclosed in square brackets, as per RFC2732.
func (c *Control) BindAddressOrLoopback(chooseHostInterface, urlSafe bool) string {
	ip := c.BindAddress
	if ip == "" && chooseHostInterface {
		if hostIP, _ := utilnet.ChooseHostInterface(); len(hostIP) > 0 {
			ip = hostIP.String()
		}
	}
	if urlSafe && utilsnet.IsIPv6String(ip) {
		return fmt.Sprintf("[%s]", ip)
	} else if ip != "" {
		return ip
	}
	return c.Loopback(urlSafe)
}

// Loopback returns an IPv4 or IPv6 loopback address, depending on whether the cluster
// service CIDRs indicate an IPv4/Dual-Stack or IPv6 only cluster. If the urlSafe
// parameter is true, IPv6 addresses are enclosed in square brackets, as per RFC2732.
func (c *Control) Loopback(urlSafe bool) string {
	if utilsnet.IsIPv6CIDR(c.ServiceIPRange) {
		if urlSafe {
			return "[::1]"
		}
		return "::1"
	}
	return "127.0.0.1"
}

type ControlRuntimeBootstrap struct {
	ETCDServerCA       string `rotate:"true"`
	ETCDServerCAKey    string `rotate:"true"`
	ETCDPeerCA         string `rotate:"true"`
	ETCDPeerCAKey      string `rotate:"true"`
	ServerCA           string `rotate:"true"`
	ServerCAKey        string `rotate:"true"`
	ClientCA           string `rotate:"true"`
	ClientCAKey        string `rotate:"true"`
	ServiceKey         string `rotate:"true"`
	PasswdFile         string
	RequestHeaderCA    string `rotate:"true"`
	RequestHeaderCAKey string `rotate:"true"`
	IPSECKey           string
	EncryptionConfig   string
	EncryptionHash     string
}

type ControlRuntime struct {
	ControlRuntimeBootstrap

	HTTPBootstrap                        bool
	APIServerReady                       <-chan struct{}
	ContainerRuntimeReady                <-chan struct{}
	ETCDReady                            <-chan struct{}
	StartupHooksWg                       *sync.WaitGroup
	ClusterControllerStarts              map[string]leader.Callback
	LeaderElectedClusterControllerStarts map[string]leader.Callback

	ClientKubeAPICert string
	ClientKubeAPIKey  string
	NodePasswdFile    string

	SigningClientCA   string
	SigningServerCA   string
	ServiceCurrentKey string

	KubeConfigAdmin           string
	KubeConfigSupervisor      string
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

	EgressSelectorConfig  string
	CloudControllerConfig string

	ClientAuthProxyCert string
	ClientAuthProxyKey  string

	ClientAdminCert           string
	ClientAdminKey            string
	ClientSupervisorCert      string
	ClientSupervisorKey       string
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

	K3s        *k3s.Factory
	Core       *core.Factory
	Event      record.EventRecorder
	EtcdConfig endpoint.ETCDConfig
}

func NewRuntime(containerRuntimeReady <-chan struct{}) *ControlRuntime {
	return &ControlRuntime{
		ContainerRuntimeReady:                containerRuntimeReady,
		ClusterControllerStarts:              map[string]leader.Callback{},
		LeaderElectedClusterControllerStarts: map[string]leader.Callback{},
	}
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
