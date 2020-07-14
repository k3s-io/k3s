package cmds

import (
	"github.com/rancher/spur/cli"
)

func NewCtrCommand(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            "ctr",
		Usage:           "Run ctr",
		SkipFlagParsing: true,
		Action:          action,
	}
}
