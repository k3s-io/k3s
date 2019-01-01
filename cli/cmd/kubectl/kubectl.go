package kubectl

import (
	"os"

	"github.com/rancher/rio/pkg/kubectl"
	"github.com/urfave/cli"
)

func NewKubectlCommand() cli.Command {
	return cli.Command{
		Name:            "kubectl",
		Usage:           "Run kubectl",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          run,
	}
}

func run(ctx *cli.Context) error {
	os.Args = append([]string{"kubectl"}, ctx.Args()...)
	kubectl.Main()
	return nil
}
