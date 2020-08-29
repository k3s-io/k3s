package cmds

import (
	"github.com/urfave/cli"
)

func NewCRICTL(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:            "crictl",
		Usage:           "Run crictl",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          action,
	}
}
