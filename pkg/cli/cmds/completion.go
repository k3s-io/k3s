package cmds

import (
	"github.com/urfave/cli/v2"
)

func NewCompletionCommand(bash, zsh func(*cli.Context) error) *cli.Command {
	installFlag := cli.BoolFlag{
		Name:  "i",
		Usage: "Install source line to rc file",
	}

	return &cli.Command{
		Name:      "completion",
		Usage:     "Install shell completion script",
		UsageText: appName + " completion [COMMAND]",
		Subcommands: []*cli.Command{
			{
				Name:      "bash",
				Usage:     "Bash completion",
				UsageText: appName + " completion bash [OPTIONS]",
				Action:    bash,
				Flags: []cli.Flag{
					&installFlag,
				},
			},
			{
				Name:      "zsh",
				Usage:     "Zsh completion",
				Action:    zsh,
				UsageText: appName + " completion zsh [OPTIONS]",
				Flags: []cli.Flag{
					&installFlag,
				},
			},
		},
	}
}
