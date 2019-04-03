package config

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

	"k8s.io/apiserver/pkg/authentication/authenticator"
)

type Node struct {
	Docker                   bool
	ContainerRuntimeEndpoint string
	NoFlannel                bool
	FlannelConf              string
	LocalAddress             string
	Containerd               Containerd
	Images                   string
	AgentConfig              Agent
	CACerts                  []byte
	ServerAddress            string
	Certificate              *tls.Certificate
}

type Containerd struct {
	Address string
	Log     string
	Root    string
	State   string
	Config  string
	Opt     string
}

type Agent struct {
	NodeName           string
	ClusterCIDR        net.IPNet
	ClusterDNS         net.IP
	ResolvConf         string
	RootDir            string
	KubeConfig         string
	NodeIP             string
	RuntimeSocket      string
	ListenAddress      string
	CACertPath         string
	CNIBinDir          string
	CNIConfDir         string
	ExtraKubeletArgs   []string
	ExtraKubeProxyArgs []string
}

type Control struct {
	AdvertisePort         int
	ListenPort            int
	ClusterSecret         string
	ClusterIPRange        *net.IPNet
	ServiceIPRange        *net.IPNet
	ClusterDNS            net.IP
	NoCoreDNS             bool
	KubeConfigOutput      string
	KubeConfigMode        string
	DataDir               string
	Skips                 []string
	ETCDEndpoints         []string
	ETCDKeyFile           string
	ETCDCertFile          string
	ETCDCAFile            string
	NoScheduler           bool
	ExtraAPIArgs          []string
	ExtraControllerArgs   []string
	ExtraSchedulerAPIArgs []string
	NoLeaderElect         bool

	Runtime *ControlRuntime `json:"-"`
}

type ControlRuntime struct {
	ServerCA           string
	ServerCAKey        string
	ClientCA           string
	ClientCAKey        string
	RequestHeaderCA    string
	RequestHeaderCAKey string

	ServingKubeAPICert  string
	ServingKubeAPIKey   string
	ClientKubeAPICert   string
	ClientKubeAPIKey    string
	ClientAuthProxyCert string
	ClientAuthProxyKey  string

	ServiceKey       string
	PasswdFile       string
	KubeConfigSystem string

	ClientToken   string
	NodeToken     string
	Handler       http.Handler
	Tunnel        http.Handler
	Authenticator authenticator.Request
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
		splitArg := strings.Split(arg, "=")
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
	return args
}
