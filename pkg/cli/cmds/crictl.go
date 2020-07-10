package cmds

import (
	"github.com/rancher/spur/cli"
)

func NewCRICTL(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            "crictl",
		Usage:           "Run crictl",
		SkipFlagParsing: true,
		Action:          action,
	}
}
