package cmds

import (
	"github.com/urfave/cli"
)

func NewKubectlCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:            "kubectl",
		Usage:           "Run kubectl",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          action,
	}
}
