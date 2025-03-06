package cmds

import (
	"github.com/urfave/cli/v2"
)

func NewCRICTL(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            "crictl",
		Usage:           "Run crictl",
		SkipFlagParsing: true,
		Action:          action,
	}
}
