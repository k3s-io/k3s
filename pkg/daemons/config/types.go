package config

import (
	"crypto/tls"
	"net"
	"net/http"

	"k8s.io/apiserver/pkg/authentication/authenticator"
)

type Node struct {
	Docker        bool
	NoFlannel     bool
	NoCoreDNS     bool
	LocalAddress  string
	AgentConfig   Agent
	CACerts       []byte
	ServerAddress string
	Certificate   *tls.Certificate
}

type Agent struct {
	NodeName           string
	ClusterCIDR        net.IPNet
	ClusterDNS         net.IP
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
	ClusterIPRange        *net.IPNet
	ServiceIPRange        *net.IPNet
	DataDir               string
	ETCDEndpoints         []string
	ETCDKeyFile           string
	ETCDCertFile          string
	ETCDCAFile            string
	NoScheduler           bool
	ExtraAPIArgs          []string
	ExtraControllerArgs   []string
	ExtraSchedulerAPIArgs []string
	NodeConfig            Node

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
}
