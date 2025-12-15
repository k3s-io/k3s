package cmds

import (
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli/v2"
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
		&cli.StringFlag{
			Name:        "kubeconfig",
			Usage:       "(cluster) Server to connect to",
			EnvVars:     []string{"KUBECONFIG"},
			Destination: &TokenConfig.Kubeconfig,
		},
	}
)

func NewTokenCommands(createFunc, deleteFunc, generateFunc, listFunc, rotateFunc func(ctx *cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            TokenCommand,
		Usage:           "Manage tokens",
		SkipFlagParsing: false,
		Subcommands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create bootstrap tokens on the server",
				Flags: append(TokenFlags, &cli.StringFlag{
					Name:        "description",
					Usage:       "A human friendly description of how this token is used",
					Destination: &TokenConfig.Description,
				}, &cli.StringSliceFlag{
					Name:        "groups",
					Usage:       "Extra groups that this token will authenticate as when used for authentication",
					Destination: &TokenConfig.Groups,
				}, &cli.DurationFlag{
					Name:        "ttl",
					Usage:       "The duration before the token is automatically deleted (e.g. 1s, 2m, 3h). If set to '0', the token will never expire",
					Value:       time.Hour * 24,
					Destination: &TokenConfig.TTL,
				}, &cli.StringSliceFlag{
					Name:        "usages",
					Usage:       "Describes the ways in which this token can be used.",
					Destination: &TokenConfig.Usages,
				}),
				SkipFlagParsing: false,
				Action:          createFunc,
			},
			{
				Name:            "delete",
				Usage:           "Delete bootstrap tokens on the server",
				Flags:           TokenFlags,
				SkipFlagParsing: false,
				Action:          deleteFunc,
			},
			{
				Name:            "generate",
				Usage:           "Generate and print a bootstrap token, but do not create it on the server",
				Flags:           TokenFlags,
				SkipFlagParsing: false,
				Action:          generateFunc,
			},
			{
				Name:  "list",
				Usage: "List bootstrap tokens on the server",
				Flags: append(TokenFlags, &cli.StringFlag{
					Name:        "output",
					Aliases:     []string{"o"},
					Value:       "text",
					Destination: &TokenConfig.Output,
				}),
				SkipFlagParsing: false,
				Action:          listFunc,
			},
			{
				Name:  "rotate",
				Usage: "Rotate original server token with a new server token",
				Flags: append(TokenFlags,
					&cli.StringFlag{
						Name:        "token",
						Aliases:     []string{"t"},
						Usage:       "Existing token used to join a server or agent to a cluster",
						Destination: &TokenConfig.Token,
						EnvVars:     []string{version.ProgramUpper + "_TOKEN"},
					},
					&cli.StringFlag{
						Name:        "server",
						Aliases:     []string{"s"},
						Usage:       "(cluster) Server to connect to",
						Destination: &TokenConfig.ServerURL,
						EnvVars:     []string{version.ProgramUpper + "_URL"},
						Value:       "https://127.0.0.1:6443",
					},
					&cli.StringFlag{
						Name:        "new-token",
						Usage:       "New token that replaces existing token",
						Destination: &TokenConfig.NewToken,
					}),
				SkipFlagParsing: false,
				Action:          rotateFunc,
			},
		},
	}
}
