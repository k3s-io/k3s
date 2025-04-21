package cmds

import (
	"context"
	"github.com/urfave/cli/v3"
)

func NewKubectlCommand(action func(ctx context.Context, cmd *cli.Command) error) *cli.Command {
	return &cli.Command{
		Name:            "kubectl",
		Usage:           "Run kubectl",
		SkipFlagParsing: true,
		Action:          action,
	}
}
