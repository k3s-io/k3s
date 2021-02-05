package cmds

import (
	"github.com/urfave/cli"
)

func NewETCDCTLCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:            "etcdctl",
		Usage:           "Run etcdctl",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          action,
	}
}
