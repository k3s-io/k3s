package cmds

import (
	"context"
	"github.com/urfave/cli/v3"
)

func NewCtrCommand(action func(ctx context.Context, cmd *cli.Command) error) *cli.Command {
	return &cli.Command{
		Name:            "ctr",
		Usage:           "Run ctr",
		SkipFlagParsing: true,
		Action:          action,
	}
}
