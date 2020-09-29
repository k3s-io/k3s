package cmds

import (
	"github.com/urfave/cli"
)

func NewContainerd(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:      "containerd",
		Usage:     "Run containerd",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          action,
	}
}
