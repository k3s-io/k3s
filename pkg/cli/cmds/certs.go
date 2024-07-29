package cmds

import (
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli"
)

const CertCommand = "certificate"

type CertRotateCA struct {
	CACertPath string
	Force      bool
}

var (
	ServicesList           cli.StringSlice
	CertRotateCAConfig     CertRotateCA
	CertRotateCommandFlags = []cli.Flag{
		DebugFlag,
		ConfigFlag,
		LogFile,
		AlsoLogToStderr,
		DataDirFlag,
		&cli.StringSliceFlag{
			Name:  "service,s",
			Usage: "List of services to manage certificates for. Options include (admin, api-server, controller-manager, scheduler, supervisor, " + version.Program + "-controller, " + version.Program + "-server, cloud-controller, etcd, auth-proxy, kubelet, kube-proxy)",
			Value: &ServicesList,
		},
	}
	CertRotateCACommandFlags = []cli.Flag{
		DataDirFlag,
		cli.StringFlag{
			Name:        "server,s",
			Usage:       "(cluster) Server to connect to",
			EnvVar:      version.ProgramUpper + "_URL",
			Value:       "https://127.0.0.1:6443",
			Destination: &ServerConfig.ServerURL,
		},
		cli.StringFlag{
			Name:        "path",
			Usage:       "Path to directory containing new CA certificates",
			Destination: &CertRotateCAConfig.CACertPath,
			Required:    true,
		},
		cli.BoolFlag{
			Name:        "force",
			Usage:       "Force certificate replacement, even if consistency checks fail",
			Destination: &CertRotateCAConfig.Force,
		},
	}
)

func NewCertCommands(check, rotate, rotateCA func(ctx *cli.Context) error) cli.Command {
	return cli.Command{
		Name:            CertCommand,
		Usage:           "Manage K3s certificates",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Subcommands: []cli.Command{
			{
				Name:            "check",
				Usage:           "Check " + version.Program + " component certificates on disk",
				SkipFlagParsing: false,
				SkipArgReorder:  true,
				Action:          check,
				Flags:           CertRotateCommandFlags,
			},
			{
				Name:            "rotate",
				Usage:           "Rotate " + version.Program + " component certificates on disk",
				SkipFlagParsing: false,
				SkipArgReorder:  true,
				Action:          rotate,
				Flags:           CertRotateCommandFlags,
			},
			{
				Name:            "rotate-ca",
				Usage:           "Write updated " + version.Program + " CA certificates to the datastore",
				SkipFlagParsing: false,
				SkipArgReorder:  true,
				Action:          rotateCA,
				Flags:           CertRotateCACommandFlags,
			},
		},
	}
}
