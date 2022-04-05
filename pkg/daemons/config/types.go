package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/authentication/authenticator"
)

const (
	FlannelBackendNone      = "none"
	FlannelBackendVXLAN     = "vxlan"
	FlannelBackendHostGW    = "host-gw"
	FlannelBackendIPSEC     = "ipsec"
	FlannelBackendWireguard = "wireguard"
	CertificateRenewDays    = 90
)

type Node struct {
	Docker                   bool
	ContainerRuntimeEndpoint string
	NoFlannel                bool
	SELinux                  bool
	FlannelBackend           string
	FlannelConfFile          string
	FlannelConfOverride      bool
	FlannelIface             *net.Interface
	FlannelIPv6Masq          bool
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
	CNIPlugin               bool
	NodeTaints              []string
	NodeLabels              []string
	ImageCredProvBinDir     string
	ImageCredProvConfig     string
	IPSECPSK                string
	StrongSwanDir           string
	PrivateRegistry         string
	SystemDefaultRegistry   string
	AirgapExtraRegistry     []string
	DisableCCM              bool
	DisableNPC              bool
	Rootless                bool
	ProtectKernelDefaults   bool
	DisableServiceLB        bool
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

// GetArgs appends extra arguments to existing arguments overriding any default options.
func GetArgs(argsMap map[string]string, extraArgs []string) []string {
	const hyphens = "--"

	// add extra args to args map to override any default option
	for _, arg := range extraArgs {
		splitArg := strings.SplitN(strings.TrimPrefix(arg, hyphens), "=", 2)
		if len(splitArg) < 2 {
			argsMap[splitArg[0]] = "true"
			continue
		}
		argsMap[splitArg[0]] = splitArg[1]
	}
	var args []string
	for arg, value := range argsMap {
		cmd := fmt.Sprintf("%s%s=%s", hyphens, strings.TrimPrefix(arg, hyphens), value)
		args = append(args, cmd)
	}
	sort.Strings(args)
	return args
}
