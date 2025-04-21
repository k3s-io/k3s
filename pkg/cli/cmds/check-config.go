package cmds

import (
	"context"

	"github.com/urfave/cli/v3"
)

func NewCheckConfigCommand(action func(ctx context.Context, cmd *cli.Command) error) *cli.Command {
	return &cli.Command{
		Name:            "check-config",
		Usage:           "Run config check",
		SkipFlagParsing: true,
		Action:          action,
	}
}
