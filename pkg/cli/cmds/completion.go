package cmds

import (
	"context"

	"github.com/urfave/cli/v3"
)

func NewCompletionCommand(action func(ctx context.Context, cmd *cli.Command) error) *cli.Command {
	return &cli.Command{
		Name:      "completion",
		Usage:     "Install shell completion script",
		UsageText: appName + " completion [SHELL] (valid shells: bash, zsh)",
		Action:    action,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "i",
				Usage: "Install source line to rc file",
			},
		},
	}
}
