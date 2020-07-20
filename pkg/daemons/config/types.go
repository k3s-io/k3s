package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"

	"github.com/rancher/kine/pkg/endpoint"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/core"
	"k8s.io/apiserver/pkg/authentication/authenticator"
)

const (
	FlannelBackendNone      = "none"
	FlannelBackendVXLAN     = "vxlan"
	FlannelBackendHostGW    = "host-gw"
	FlannelBackendIPSEC     = "ipsec"
	FlannelBackendWireguard = "wireguard"
)

type Node struct {
	Docker                   bool
	ContainerRuntimeEndpoint string
	NoFlannel                bool
	DisableSELinux           bool
	FlannelBackend           string
	FlannelConf              string
	FlannelConfOverride      bool
	FlannelIface             *net.Interface
	Containerd               Containerd
	Images                   string
	AgentConfig              Agent
	CACerts                  []byte
	Certificate              *tls.Certificate
}

type Containerd struct {
	Address  string
	Log      string
	Root     string
	State    string
	Config   string
	Opt      string
	Template string
}

type Agent struct {
	PodManifests            string
	NodeName                string
	NodeConfigPath          string
	ServingKubeletCert      string
	ServingKubeletKey       string
	ClusterCIDR             net.IPNet
	ClusterDNS              net.IP
	ClusterDomain           string
	ResolvConf              string
	RootDir                 string
	KubeConfigKubelet       string
	KubeConfigKubeProxy     string
	KubeConfigK3sController string
	NodeIP                  string
	NodeExternalIP          string
	RuntimeSocket           string
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
	IPSECPSK                string
	StrongSwanDir           string
	PrivateRegistry         string
	DisableCCM              bool
	DisableNPC              bool
	DisableKubeProxy        bool
	Rootless                bool
	ProtectKernelDefaults   bool
}

type Control struct {
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
	ClusterIPRange           *net.IPNet
	ServiceIPRange           *net.IPNet
	ClusterDNS               net.IP
	ClusterDomain            string
	NoCoreDNS                bool
	KubeConfigOutput         string
	KubeConfigMode           string
	DataDir                  string
	Skips                    map[string]bool
	Disables                 map[string]bool
	Datastore                endpoint.Config
	NoScheduler              bool
	ExtraAPIArgs             []string
	ExtraControllerArgs      []string
	ExtraCloudControllerArgs []string
	ExtraSchedulerAPIArgs    []string
	NoLeaderElect            bool
	JoinURL                  string
	FlannelBackend           string
	IPSECPSK                 string
	DefaultLocalStoragePath  string
	DisableCCM               bool
	DisableNPC               bool
	DisableKubeProxy         bool
	ClusterInit              bool
	ClusterReset             bool
	EncryptSecrets           bool
	TLSMinVersion            uint16
	TLSCipherSuites          []uint16

	BindAddress string
	SANs        []string

	Runtime *ControlRuntime `json:"-"`
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
	ControlRuntimeBootstrap

	HTTPBootstrap          bool
	APIServerReady         <-chan struct{}
	ETCDReady              <-chan struct{}
	ClusterControllerStart func(ctx context.Context) error

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

	Core *core.Factory
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

func GetArgsList(argsMap map[string]string, extraArgs []string) []string {
	// add extra args to args map to override any default option
	for _, arg := range extraArgs {
		splitArg := strings.SplitN(arg, "=", 2)
		if len(splitArg) < 2 {
			argsMap[splitArg[0]] = "true"
			continue
		}
		argsMap[splitArg[0]] = splitArg[1]
	}
	var args []string
	for arg, value := range argsMap {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		args = append(args, cmd)
	}
	sort.Strings(args)
	return args
}
