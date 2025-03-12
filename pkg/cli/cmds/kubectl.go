package cmds

import (
	"github.com/urfave/cli/v2"
)

func NewKubectlCommand(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            "kubectl",
		Usage:           "Run kubectl",
		SkipFlagParsing: true,
		Action:          action,
	}
}
