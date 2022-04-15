package cmds

import (
	"fmt"

	"github.com/k3s-io/k3s/pkg/cli/completion"
	"github.com/urfave/cli"
)

func NewCompletionCommand() cli.Command {
	return cli.Command{
		Name:      "completion",
		Usage:     "Install shell completion script",
		UsageText: appName + " completion [SHELL] (valid shells: bash, zsh)",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("must provide a valid SHELL argument")
			}
			return completion.ShellCompletionInstall(appName, ctx.Args()[0])
		},
	}
}
