package cmds

import (
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli/v2"
)

const SecretsEncryptCommand = "secrets-encrypt"

var (
	forceFlag = &cli.BoolFlag{
		Name:        "force",
		Aliases:     []string{"f"},
		Usage:       "Force this stage.",
		Destination: &ServerConfig.EncryptForce,
	}
	EncryptFlags = []cli.Flag{
		DataDirFlag,
		ServerToken,
		&cli.StringFlag{
			Name:        "server",
			Aliases:     []string{"s"},
			Usage:       "(cluster) Server to connect to",
			EnvVars:     []string{version.ProgramUpper + "_URL"},
			Value:       "https://127.0.0.1:6443",
			Destination: &ServerConfig.ServerURL,
		},
	}
)

func NewSecretsEncryptCommands(status, enable, disable, prepare, rotate, reencrypt, rotateKeys func(ctx *cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:  SecretsEncryptCommand,
		Usage: "Control secrets encryption and keys rotation",
		Subcommands: []*cli.Command{
			{
				Name:   "status",
				Usage:  "Print current status of secrets encryption",
				Action: status,
				Flags: append(EncryptFlags, &cli.StringFlag{
					Name:        "output",
					Aliases:     []string{"o"},
					Usage:       "Status format (valid values: text, json)",
					Destination: &ServerConfig.EncryptOutput,
					Value:       "text",
				}),
			},
			{
				Name:   "enable",
				Usage:  "Enable secrets encryption",
				Action: enable,
				Flags:  EncryptFlags,
			},
			{
				Name:   "disable",
				Usage:  "Disable secrets encryption",
				Action: disable,
				Flags:  EncryptFlags,
			},
			{
				Name:   "prepare",
				Usage:  "Prepare for encryption keys rotation",
				Action: prepare,
				Flags:  append(EncryptFlags, forceFlag),
			},
			{
				Name:   "rotate",
				Usage:  "Rotate secrets encryption keys",
				Action: rotate,
				Flags:  append(EncryptFlags, forceFlag),
			},
			{
				Name:   "reencrypt",
				Usage:  "Reencrypt all data with new encryption key",
				Action: reencrypt,
				Flags: append(EncryptFlags,
					forceFlag,
					&cli.BoolFlag{
						Name:        "skip",
						Usage:       "Skip removing old key",
						Destination: &ServerConfig.EncryptSkip,
					}),
			},
			{
				Name:   "rotate-keys",
				Usage:  "Dynamically rotates secrets encryption keys and re-encrypt secrets",
				Action: rotateKeys,
				Flags:  EncryptFlags,
			},
		},
	}
}
