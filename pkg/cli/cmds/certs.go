package cmds

import (
	"github.com/rancher/k3s/pkg/version"
	"github.com/urfave/cli"
)

const CertCommand = "cert"

var (
	ComponentList    cli.StringSlice
	CertCommandFlags = []cli.Flag{
		DebugFlag,
		ConfigFlag,
		LogFile,
		AlsoLogToStderr,
		cli.StringFlag{
			Name:        "data-dir,d",
			Usage:       "(data) Folder to hold state default /var/lib/rancher/" + version.Program + " or ${HOME}/.rancher/" + version.Program + " if not root",
			Destination: &ServerConfig.DataDir,
		},
		cli.StringSliceFlag{
			Name:  "service,s",
			Usage: "List of services to rotate their certificates the options include (admin, api-server, controller-manager, scheduler, " + version.Program + "-controller, cloud-controller, etcd, auth-proxy, kubelet, kube-proxy)",
			Value: &ComponentList,
		},
	}
)

func NewCertCommand(action func(*cli.Context) error, subcommands []cli.Command) cli.Command {
	return cli.Command{
		Name:            CertCommand,
		Usage:           "Certificates management",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          action,
		Subcommands:     subcommands,
		Flags:           CertCommandFlags,
	}
}

func NewCertSubcommands(rotate func(ctx *cli.Context) error) []cli.Command {
	return []cli.Command{
		{
			Name:            "rotate",
			Usage:           "Certificate rotation",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          rotate,
			Flags:           CertCommandFlags,
		},
	}
}
