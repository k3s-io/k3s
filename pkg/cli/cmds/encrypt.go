package cmds

import (
	"github.com/rancher/k3s/pkg/version"
	"github.com/urfave/cli"
)

const EncryptCommand = "encrypt"

var EncryptFlags = []cli.Flag{
	cli.StringFlag{
		Name:        "data-dir,d",
		Usage:       "(data) Folder to hold state default /var/lib/rancher/" + version.Program + " or ${HOME}/.rancher/" + version.Program + " if not root",
		Destination: &ServerConfig.DataDir,
	},
}

func NewEncryptCommand(action func(*cli.Context) error, subcommands []cli.Command) cli.Command {
	return cli.Command{
		Name:            EncryptCommand,
		Usage:           "(experimental) Print current status of secrets encryption",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          action,
		Flags:           EncryptFlags,
		Subcommands:     subcommands,
	}
}

func NewEncryptSubcommands(prepare, rotate, reencrypt func(ctx *cli.Context) error) []cli.Command {
	return []cli.Command{
		{
			Name:            "prepare",
			Usage:           "(experimental) Prepare for encryption keys rotation",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          prepare,
			Flags:           EncryptFlags,
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
				Destination: &ServerConfig.EncryptForceRotation,
			}),
		},
		{
			Name:            "reencrypt",
			Usage:           "(experimental) Reencrypt all data with new encryption key",
			SkipFlagParsing: false,
			SkipArgReorder:  true,
			Action:          reencrypt,
			Flags:           EncryptFlags,
		},
	}
}
