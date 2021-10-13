package main

import (
	"context"
	"errors"
	"os"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/secretsencrypt"
	"github.com/rancher/k3s/pkg/configfilearg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewSecretsEncryptCommand(secretsencrypt.Run,
			cmds.NewSecretsEncryptSubcommands(
				secretsencrypt.Status,
				secretsencrypt.Prepare,
				secretsencrypt.Rotate,
				secretsencrypt.Reencrypt),
		),
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
