package cmds

import (
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli"
)

const SecretsEncryptCommand = "secrets-encrypt"

var (
	forceFlag = &cli.BoolFlag{
		Name:        "f,force",
		Usage:       "Force this stage.",
		Destination: &ServerConfig.EncryptForce,
	}
	EncryptFlags = []cli.Flag{
		DataDirFlag,
		ServerToken,
		&cli.StringFlag{
			Name:        "server, s",
			Usage:       "(cluster) Server to connect to",
			EnvVar:      version.ProgramUpper + "_URL",
			Value:       "https://127.0.0.1:6443",
			Destination: &ServerConfig.ServerURL,
		},
	}
)

func NewSecretsEncryptCommands(status, enable, disable, prepare, rotate, reencrypt, rotateKeys func(ctx *cli.Context) error) cli.Command {
	return cli.Command{
		Name:           SecretsEncryptCommand,
		Usage:          "Control secrets encryption and keys rotation",
		SkipArgReorder: true,
		Subcommands: []cli.Command{
			{
				Name:           "status",
				Usage:          "Print current status of secrets encryption",
				SkipArgReorder: true,
				Action:         status,
				Flags: append(EncryptFlags, &cli.StringFlag{
					Name:        "output,o",
					Usage:       "Status format. Default: text. Optional: json",
					Destination: &ServerConfig.EncryptOutput,
				}),
			},
			{
				Name:           "enable",
				Usage:          "Enable secrets encryption",
				SkipArgReorder: true,
				Action:         enable,
				Flags:          EncryptFlags,
			},
			{
				Name:           "disable",
				Usage:          "Disable secrets encryption",
				SkipArgReorder: true,
				Action:         disable,
				Flags:          EncryptFlags,
			},
			{
				Name:           "prepare",
				Usage:          "Prepare for encryption keys rotation",
				SkipArgReorder: true,
				Action:         prepare,
				Flags:          append(EncryptFlags, forceFlag),
			},
			{
				Name:           "rotate",
				Usage:          "Rotate secrets encryption keys",
				SkipArgReorder: true,
				Action:         rotate,
				Flags:          append(EncryptFlags, forceFlag),
			},
			{
				Name:           "reencrypt",
				Usage:          "Reencrypt all data with new encryption key",
				SkipArgReorder: true,
				Action:         reencrypt,
				Flags: append(EncryptFlags,
					forceFlag,
					&cli.BoolFlag{
						Name:        "skip",
						Usage:       "Skip removing old key",
						Destination: &ServerConfig.EncryptSkip,
					}),
			},
			{
				Name:           "rotate-keys",
				Usage:          "(experimental) Dynamically rotates secrets encryption keys and re-encrypt secrets",
				SkipArgReorder: true,
				Action:         rotateKeys,
				Flags:          EncryptFlags,
			},
		},
	}
}
