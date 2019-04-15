package cmds

import (
	"github.com/urfave/cli"
)

type Server struct {
	Log                 string
	ClusterCIDR         string
	ClusterSecret       string
	ServiceCIDR         string
	ClusterDNS          string
	ClusterDomain       string
	HTTPSPort           int
	HTTPPort            int
	DataDir             string
	DisableAgent        bool
	KubeConfigOutput    string
	KubeConfigMode      string
	KnownIPs            cli.StringSlice
	BindAddress         string
	ExtraAPIArgs        cli.StringSlice
	ExtraSchedulerArgs  cli.StringSlice
	ExtraControllerArgs cli.StringSlice
	Rootless            bool
}

var ServerConfig Server

func NewServerCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:      "server",
		Usage:     "Run management server",
		UsageText: appName + " server [OPTIONS]",
		Action:    action,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:        "bind-address",
				Usage:       "k3s bind address (default: localhost)",
				Destination: &ServerConfig.BindAddress,
			},
			cli.IntFlag{
				Name:        "https-listen-port",
				Usage:       "HTTPS listen port",
				Value:       6443,
				Destination: &ServerConfig.HTTPSPort,
			},
			cli.IntFlag{
				Name:        "http-listen-port",
				Usage:       "HTTP listen port (for /healthz, HTTPS redirect, and port for TLS terminating LB)",
				Value:       0,
				Destination: &ServerConfig.HTTPPort,
			},
			cli.StringFlag{
				Name:        "data-dir,d",
				Usage:       "Folder to hold state default /var/lib/rancher/k3s or ${HOME}/.rancher/k3s if not root",
				Destination: &ServerConfig.DataDir,
			},
			cli.BoolFlag{
				Name:        "disable-agent",
				Usage:       "Do not run a local agent and register a local kubelet",
				Destination: &ServerConfig.DisableAgent,
			},
			cli.StringFlag{
				Name:        "log,l",
				Usage:       "Log to file",
				Destination: &ServerConfig.Log,
			},
			cli.StringFlag{
				Name:        "cluster-cidr",
				Usage:       "Network CIDR to use for pod IPs",
				Destination: &ServerConfig.ClusterCIDR,
				Value:       "10.42.0.0/16",
			},
			cli.StringFlag{
				Name:        "cluster-secret",
				Usage:       "Shared secret used to bootstrap a cluster",
				Destination: &ServerConfig.ClusterSecret,
				EnvVar:      "K3S_CLUSTER_SECRET",
			},
			cli.StringFlag{
				Name:        "service-cidr",
				Usage:       "Network CIDR to use for services IPs",
				Destination: &ServerConfig.ServiceCIDR,
				Value:       "10.43.0.0/16",
			},
			cli.StringFlag{
				Name:        "cluster-dns",
				Usage:       "Cluster IP for coredns service. Should be in your service-cidr range",
				Destination: &ServerConfig.ClusterDNS,
				Value:       "",
			},
			cli.StringFlag{
				Name:        "cluster-domain",
				Usage:       "Cluster Domain",
				Destination: &ServerConfig.ClusterDomain,
				Value:       "cluster.local",
			},
			cli.StringSliceFlag{
				Name:  "no-deploy",
				Usage: "Do not deploy packaged components (valid items: coredns, servicelb, traefik)",
			},
			cli.StringFlag{
				Name:        "write-kubeconfig,o",
				Usage:       "Write kubeconfig for admin client to this file",
				Destination: &ServerConfig.KubeConfigOutput,
				EnvVar:      "K3S_KUBECONFIG_OUTPUT",
			},
			cli.StringFlag{
				Name:        "write-kubeconfig-mode",
				Usage:       "Write kubeconfig with this mode",
				Destination: &ServerConfig.KubeConfigMode,
				EnvVar:      "K3S_KUBECONFIG_MODE",
			},
			cli.StringSliceFlag{
				Name:  "tls-san",
				Usage: "Add additional hostname or IP as a Subject Alternative Name in the TLS cert",
				Value: &ServerConfig.KnownIPs,
			},
			cli.StringSliceFlag{
				Name:  "kube-apiserver-arg",
				Usage: "Customized flag for kube-apiserver process",
				Value: &ServerConfig.ExtraAPIArgs,
			},
			cli.StringSliceFlag{
				Name:  "kube-scheduler-arg",
				Usage: "Customized flag for kube-scheduler process",
				Value: &ServerConfig.ExtraSchedulerArgs,
			},
			cli.StringSliceFlag{
				Name:  "kube-controller-arg",
				Usage: "Customized flag for kube-controller-manager process",
				Value: &ServerConfig.ExtraControllerArgs,
			},
			cli.BoolFlag{
				Name:        "rootless",
				Usage:       "(experimental) Run rootless",
				Destination: &ServerConfig.Rootless,
			},
			NodeIPFlag,
			NodeNameFlag,
			DockerFlag,
			FlannelFlag,
			FlannelIfaceFlag,
			CRIEndpointFlag,
			ResolvConfFlag,
			ExtraKubeletArgs,
			ExtraKubeProxyArgs,
		},
	}
}
