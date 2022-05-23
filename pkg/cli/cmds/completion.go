package cmds

import (
	"github.com/urfave/cli"
)

func NewCompletionCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:      "completion",
		Usage:     "Install shell completion script",
		UsageText: appName + " completion [SHELL] (valid shells: bash, zsh)",
		Action:    action,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "i",
				Usage: "Install source line to rc file",
			},
		},
	}
}
