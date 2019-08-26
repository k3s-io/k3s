package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"

	"github.com/rancher/kine/pkg/endpoint"

	"k8s.io/apiserver/pkg/authentication/authenticator"
)

type Node struct {
	Docker                   bool
	ContainerRuntimeEndpoint string
	NoFlannel                bool
	FlannelConf              string
	FlannelIface             *net.Interface
	Containerd               Containerd
	Images                   string
	AgentConfig              Agent
	CACerts                  []byte
	ServerAddress            string
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
	NodeName            string
	ClientKubeletCert   string
	ClientKubeletKey    string
	ClientKubeProxyCert string
	ClientKubeProxyKey  string
	ServingKubeletCert  string
	ServingKubeletKey   string
	ClusterCIDR         net.IPNet
	ClusterDNS          net.IP
	ClusterDomain       string
	ResolvConf          string
	RootDir             string
	KubeConfigNode      string
	KubeConfigKubelet   string
	KubeConfigKubeProxy string
	NodeIP              string
	RuntimeSocket       string
	ListenAddress       string
	ClientCA            string
	CNIBinDir           string
	CNIConfDir          string
	ExtraKubeletArgs    []string
	ExtraKubeProxyArgs  []string
	PauseImage          string
	CNIPlugin           bool
	NodeTaints          []string
	NodeLabels          []string
}

type Control struct {
	AdvertisePort         int
	AdvertiseIP           string
	ListenPort            int
	HTTPSPort             int
	ClusterSecret         string
	ClusterIPRange        *net.IPNet
	ServiceIPRange        *net.IPNet
	ClusterDNS            net.IP
	ClusterDomain         string
	NoCoreDNS             bool
	KubeConfigOutput      string
	KubeConfigMode        string
	DataDir               string
	Skips                 []string
	BootstrapReadOnly     bool
	Storage               endpoint.Config
	NoScheduler           bool
	ExtraAPIArgs          []string
	ExtraControllerArgs   []string
	ExtraSchedulerAPIArgs []string
	NoLeaderElect         bool

	Runtime *ControlRuntime `json:"-"`
}

type ControlRuntimeBootstrap struct {
	ServerCA           string
	ServerCAKey        string
	ClientCA           string
	ClientCAKey        string
	ServiceKey         string
	PasswdFile         string
	RequestHeaderCA    string
	RequestHeaderCAKey string
	ClientKubeletKey   string
	ClientKubeProxyKey string
	ServingKubeletKey  string
}

type ControlRuntime struct {
	ControlRuntimeBootstrap

	ClientKubeAPICert string
	ClientKubeAPIKey  string
	NodePasswdFile    string

	KubeConfigAdmin      string
	KubeConfigController string
	KubeConfigScheduler  string
	KubeConfigAPIServer  string

	ServingKubeAPICert string
	ServingKubeAPIKey  string
	ClientToken        string
	NodeToken          string
	Handler            http.Handler
	Tunnel             http.Handler
	Authenticator      authenticator.Request

	ClientAuthProxyCert string
	ClientAuthProxyKey  string

	ClientAdminCert      string
	ClientAdminKey       string
	ClientControllerCert string
	ClientControllerKey  string
	ClientSchedulerCert  string
	ClientSchedulerKey   string
	ClientKubeProxyCert  string
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
