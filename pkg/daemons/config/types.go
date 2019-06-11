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
	FlannelIface             *net.Interface
	LocalAddress             string
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
	NodeName           string
	NodeCertFile       string
	NodeKeyFile        string
	ClusterCIDR        net.IPNet
	ClusterDNS         net.IP
	ClusterDomain      string
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
	PauseImage         string
	CNIPlugin          bool
	NodeTaints         []string
	NodeLabels         []string
}

type Control struct {
	AdvertisePort         int
	ListenPort            int
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
	StorageBackend        string
	StorageEndpoint       string
	StorageCAFile         string
	StorageCertFile       string
	StorageKeyFile        string
	NoScheduler           bool
	ExtraAPIArgs          []string
	ExtraControllerArgs   []string
	ExtraSchedulerAPIArgs []string
	NoLeaderElect         bool

	Runtime *ControlRuntime `json:"-"`
}

type ControlRuntime struct {
	TLSCert          string
	TLSKey           string
	TLSCA            string
	TLSCAKey         string
	TokenCA          string
	TokenCAKey       string
	ServiceKey       string
	PasswdFile       string
	KubeConfigSystem string

	NodeCert      string
	NodeKey       string
	ClientToken   string
	NodeToken     string
	Handler       http.Handler
	Tunnel        http.Handler
	Authenticator authenticator.Request

	RequestHeaderCA     string
	RequestHeaderCAKey  string
	ClientAuthProxyCert string
	ClientAuthProxyKey  string
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
	return args
}
