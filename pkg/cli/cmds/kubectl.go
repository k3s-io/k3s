package cmds

import (
	"github.com/rancher/spur/cli"
)

func NewKubectlCommand(action func(*cli.Context) error) *cli.Command {
	return &cli.Command{
		Name:            "kubectl",
		Usage:           "Run kubectl",
		SkipFlagParsing: true,
		Action:          action,
	}
}
