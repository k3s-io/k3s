package cmds

import (
	"github.com/urfave/cli"
)

func NewCtrCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:            "ctr",
		Usage:           "Run ctr",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          action,
	}
}
