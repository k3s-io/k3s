package cmds

import (
	"github.com/rancher/k3s/pkg/version"
	"github.com/urfave/cli"
)

const SecretsEncryptCommand = "secrets-encrypt"

var EncryptFlags = []cli.Flag{
	cli.StringFlag{
		Name:        "data-dir,d",
		Usage:       "(data) Folder to hold state default /var/lib/rancher/" + version.Program + " or ${HOME}/.rancher/" + version.Program + " if not root",
		Destination: &ServerConfig.DataDir,
	},
	cli.StringFlag{
		Name:        "token,t",
		Usage:       "(cluster) Token to use for authentication",
		EnvVar:      version.ProgramUpper + "_TOKEN",
		Destination: &ServerConfig.Token,
	},
}

func NewSecretsEncryptCommand(action func(*cli.Context) error, subcommands []cli.Command) cli.Command {
	return cli.Command{
		Name:            SecretsEncryptCommand,
		Usage:           "(experimental) Control secrets encryption and keys rotation",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          action,
		Subcommands:     subcommands,
	}
}

func NewSecretsEncryptSubcommands(status, enable, disable, prepare, rotate, reencrypt func(ctx *cli.Context) error) []cli.Command {
	return []cli.Command{
		{
			Name:            "status",
			Usage:           "(experimental) Print current status of secrets encryption",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          status,
			Flags:           EncryptFlags,
		},
		{
			Name:            "enable",
			Usage:           "(experimental) Enable secrets encryption",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          enable,
			Flags:           EncryptFlags,
		},
		{
			Name:            "disable",
			Usage:           "(experimental) Disable secrets encryption",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          disable,
			Flags:           EncryptFlags,
		},
		{
			Name:            "prepare",
			Usage:           "(experimental) Prepare for encryption keys rotation",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          prepare,
			Flags: append(EncryptFlags, &cli.BoolFlag{
				Name:        "f,force",
				Usage:       "Force preparation.",
				Destination: &ServerConfig.EncryptForce,
			}),
		},
		{
			Name:            "rotate",
			Usage:           "(experimental) Rotate secrets encryption keys",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          rotate,
			Flags: append(EncryptFlags, &cli.BoolFlag{
				Name:        "f,force",
				Usage:       "Force key rotation.",
				Destination: &ServerConfig.EncryptForce,
			}),
		},
		{
			Name:            "reencrypt",
			Usage:           "(experimental) Reencrypt all data with new encryption key",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          reencrypt,
			Flags: append(EncryptFlags, &cli.BoolFlag{
				Name:        "f,force",
				Usage:       "Force secrets reencryption.",
				Destination: &ServerConfig.EncryptForce,
			}),
		},
	}
}
