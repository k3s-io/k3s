package cmds

import (
	"context"

	"github.com/urfave/cli/v3"
)

func NewCRICTL(action func(ctx context.Context, cmd *cli.Command) error) *cli.Command {
	return &cli.Command{
		Name:            "crictl",
		Usage:           "Run crictl",
		SkipFlagParsing: true,
		Action:          action,
	}
}
