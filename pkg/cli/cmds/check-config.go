package cmds

import (
	"github.com/urfave/cli/v2"
)

func NewCheckConfigCommand(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            "check-config",
		Usage:           "Run config check",
		SkipFlagParsing: true,
		Action:          action,
	}
}
