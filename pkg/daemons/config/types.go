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
	FlannelIface             *net.Interface
	Certificate              *tls.Certificate
	Images                   string
	Token                    string
	FlannelBackend           string
	FlannelConf              string
	ContainerRuntimeEndpoint string
	Containerd               Containerd
	AgentConfig              Agent
	ServerHTTPSPort          int
	FlannelConfOverride      bool
	SELinux                  bool
	NoFlannel                bool
	Docker                   bool
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
	ServiceCIDR             *net.IPNet
	ClusterCIDR             *net.IPNet
	PauseImage              string
	ServingKubeletCert      string
	ServingKubeletKey       string
	NodeConfigPath          string
	PodManifests            string
	ImageCredProvBinDir     string
	NodeName                string
	SystemDefaultRegistry   string
	PrivateRegistry         string
	StrongSwanDir           string
	IPSECPSK                string
	ResolvConf              string
	RootDir                 string
	KubeConfigKubelet       string
	KubeConfigKubeProxy     string
	KubeConfigK3sController string
	NodeIP                  string
	ImageCredProvConfig     string
	NodeExternalIP          string
	Snapshotter             string
	RuntimeSocket           string
	ImageServiceSocket      string
	ListenAddress           string
	ClientCA                string
	CNIBinDir               string
	CNIConfDir              string
	ClusterDomain           string
	ClusterDNSs             []net.IP
	ExtraKubeletArgs        []string
	NodeExternalIPs         []net.IP
	ClusterCIDRs            []*net.IPNet
	NodeTaints              []string
	NodeLabels              []string
	ClusterDNS              net.IP
	NodeIPs                 []net.IP
	ServiceCIDRs            []*net.IPNet
	ExtraKubeProxyArgs      []string
	AirgapExtraRegistry     []string
	ServiceNodePortRange    utilnet.PortRange
	EnableIPv6              bool
	CNIPlugin               bool
	DisableCCM              bool
	DisableNPC              bool
	Rootless                bool
	ProtectKernelDefaults   bool
	DisableServiceLB        bool
}

type Control struct {
	ClusterIPRange           *net.IPNet
	Disables                 map[string]bool
	Skips                    map[string]bool
	Runtime                  *ControlRuntime `json:"-"`
	ServiceIPRange           *net.IPNet
	ServiceNodePortRange     *utilnet.PortRange
	Datastore                endpoint.Config
	IPSECPSK                 string
	Token                    string `json:"-"`
	EtcdS3EndpointCA         string
	APIServerBindAddress     string
	EtcdS3AccessKey          string
	EtcdSnapshotDir          string
	EtcdS3SecretKey          string
	EtcdS3BucketName         string
	ClusterDomain            string
	PrivateIP                string
	EtcdS3Endpoint           string
	KubeConfigOutput         string
	KubeConfigMode           string
	DataDir                  string
	ClusterResetRestorePath  string
	EtcdSnapshotCron         string
	AdvertiseIP              string
	EtcdS3Folder             string
	FlannelBackend           string
	ServerNodeName           string
	EtcdS3Region             string
	SystemDefaultRegistry    string
	BindAddress              string
	JoinURL                  string
	DefaultLocalStoragePath  string
	AgentToken               string `json:"-"`
	EtcdSnapshotName         string
	ClusterIPRanges          []*net.IPNet
	ExtraSchedulerAPIArgs    []string
	ExtraCloudControllerArgs []string
	TLSCipherSuites          []uint16
	ExtraAPIArgs             []string
	SANs                     []string
	ClusterDNSs              []net.IP
	ExtraEtcdArgs            []string
	ServiceIPRanges          []*net.IPNet
	ClusterDNS               net.IP
	ExtraControllerArgs      []string
	// The port which kube-apiserver runs on
	APIServerPort int
	// The port which custom k3s API runs on
	SupervisorPort        int
	EtcdSnapshotRetention int
	// The port which kubectl clients can access k8s
	HTTPSPort                int
	EtcdS3Timeout            time.Duration
	AdvertisePort            int
	TLSMinVersion            uint16
	EtcdExposeMetrics        bool
	EtcdDisableSnapshots     bool
	EncryptSecrets           bool
	EtcdS3                   bool
	ClusterReset             bool
	ClusterInit              bool
	EtcdS3SkipSSLVerify      bool
	DisableETCD              bool
	DisableScheduler         bool
	DisableControllerManager bool
	DisableAPIServer         bool
	DisableKubeProxy         bool
	DisableHelmController    bool
	EtcdS3Insecure           bool
	DisableNPC               bool
	NoLeaderElect            bool
	NoCoreDNS                bool
	DisableServiceLB         bool
	DisableCCM               bool
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
}

type ControlRuntime struct {
	Authenticator                       authenticator.Request
	Tunnel                              http.Handler
	Handler                             http.Handler
	APIServer                           http.Handler
	Core                                *core.Factory
	ClusterControllerStart              func(ctx context.Context) error
	LeaderElectedClusterControllerStart func(ctx context.Context) error
	APIServerReady                      <-chan struct{}
	AgentReady                          <-chan struct{}
	ETCDReady                           <-chan struct{}
	ControlRuntimeBootstrap
	ServerToken               string
	KubeConfigScheduler       string
	KubeConfigAPIServer       string
	KubeConfigCloudController string
	ServingKubeAPICert        string
	ServingKubeAPIKey         string
	ServingKubeletKey         string
	KubeConfigController      string
	AgentToken                string
	KubeConfigAdmin           string
	NodePasswdFile            string
	ClientKubeAPIKey          string
	ClientKubeAPICert         string
	ClientAuthProxyCert       string
	ClientAuthProxyKey        string
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
	ServerETCDCert            string
	ClientETCDKey             string
	PeerServerClientETCDCert  string
	PeerServerClientETCDKey   string
	ClientETCDCert            string
	ServerETCDKey             string
	HTTPBootstrap             bool
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
