package encrypt

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	return nil
}

func Prepare(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return nil
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return nil
}

func Reencrypt(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return nil
}
