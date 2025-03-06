package cmds

import (
	"github.com/urfave/cli/v2"
)

func NewCtrCommand(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            "ctr",
		Usage:           "Run ctr",
		SkipFlagParsing: true,
		Action:          action,
	}
}
