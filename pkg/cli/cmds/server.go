package cmds

import (
	"github.com/urfave/cli"
)

type Server struct {
	Log              string
	ClusterCIDR      string
	ClusterSecret    string
	ServiceCIDR      string
	HTTPSPort        int
	HTTPPort         int
	DataDir          string
	DisableAgent     bool
	KubeConfigOutput string
	KubeConfigMode   string
}

var ServerConfig Server

func NewServerCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:      "server",
		Usage:     "Run management server",
		UsageText: appName + " server [OPTIONS]",
		Action:    action,
		Flags: []cli.Flag{
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
			NodeIPFlag,
			NodeNameFlag,
			DockerFlag,
			FlannelFlag,
		},
	}
}
