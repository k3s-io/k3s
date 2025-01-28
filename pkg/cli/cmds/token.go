package cmds

import (
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli"
)

const TokenCommand = "token"

// Config holds CLI values for the token subcommands
type Token struct {
	Description string
	Kubeconfig  string
	ServerURL   string
	Token       string
	NewToken    string
	Output      string
	Groups      cli.StringSlice
	Usages      cli.StringSlice
	TTL         time.Duration
}

var (
	TokenConfig = Token{}
	TokenFlags  = []cli.Flag{
		DataDirFlag,
		cli.StringFlag{
			Name:        "kubeconfig",
			Usage:       "(cluster) Server to connect to",
			EnvVar:      "KUBECONFIG",
			Destination: &TokenConfig.Kubeconfig,
		},
	}
)

func NewTokenCommands(create, delete, generate, list, rotate func(ctx *cli.Context) error) cli.Command {
	return cli.Command{
		Name:            TokenCommand,
		Usage:           "Manage tokens",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Subcommands: []cli.Command{
			{
				Name:  "create",
				Usage: "Create bootstrap tokens on the server",
				Flags: append(TokenFlags, &cli.StringFlag{
					Name:        "description",
					Usage:       "A human friendly description of how this token is used",
					Destination: &TokenConfig.Description,
				}, &cli.StringSliceFlag{
					Name:  "groups",
					Usage: "Extra groups that this token will authenticate as when used for authentication",
					Value: &TokenConfig.Groups,
				}, &cli.DurationFlag{
					Name:        "ttl",
					Usage:       "The duration before the token is automatically deleted (e.g. 1s, 2m, 3h). If set to '0', the token will never expire",
					Value:       time.Hour * 24,
					Destination: &TokenConfig.TTL,
				}, &cli.StringSliceFlag{
					Name:  "usages",
					Usage: "Describes the ways in which this token can be used.",
					Value: &TokenConfig.Usages,
				}),
				SkipFlagParsing: false,
				SkipArgReorder:  true,
				Action:          create,
			},
			{
				Name:            "delete",
				Usage:           "Delete bootstrap tokens on the server",
				Flags:           TokenFlags,
				SkipFlagParsing: false,
				SkipArgReorder:  true,
				Action:          delete,
			},
			{
				Name:            "generate",
				Usage:           "Generate and print a bootstrap token, but do not create it on the server",
				Flags:           TokenFlags,
				SkipFlagParsing: false,
				SkipArgReorder:  true,
				Action:          generate,
			},
			{
				Name:  "list",
				Usage: "List bootstrap tokens on the server",
				Flags: append(TokenFlags, &cli.StringFlag{
					Name:        "output,o",
					Value:       "text",
					Destination: &TokenConfig.Output,
				}),
				SkipFlagParsing: false,
				SkipArgReorder:  true,
				Action:          list,
			},
			{
				Name:  "rotate",
				Usage: "Rotate original server token with a new server token",
				Flags: append(TokenFlags,
					&cli.StringFlag{
						Name:        "token,t",
						Usage:       "Existing token used to join a server or agent to a cluster",
						Destination: &TokenConfig.Token,
						EnvVar:      version.ProgramUpper + "_TOKEN",
					},
					&cli.StringFlag{
						Name:        "server, s",
						Usage:       "(cluster) Server to connect to",
						Destination: &TokenConfig.ServerURL,
						EnvVar:      version.ProgramUpper + "_URL",
						Value:       "https://127.0.0.1:6443",
					},
					&cli.StringFlag{
						Name:        "new-token",
						Usage:       "New token that replaces existing token",
						Destination: &TokenConfig.NewToken,
					}),
				SkipFlagParsing: false,
				SkipArgReorder:  true,
				Action:          rotate,
			},
		},
	}
}
