package cmds

import "github.com/urfave/cli"

func NewKillAllCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:            "k3s-killall",
		Usage:           "Run k3s-killall.sh script",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          action,
	}
}
