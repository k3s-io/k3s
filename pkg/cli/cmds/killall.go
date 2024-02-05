package cmds

import "github.com/urfave/cli"

func NewKillAllCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:            "killall",
		Usage:           "Kill all K3s and associated child processes",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          action,
	}
}
