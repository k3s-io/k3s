package main

import (
	"context"
	"errors"
	"os"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/secretsencrypt"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewSecretsEncryptCommands(
			secretsencrypt.Status,
			secretsencrypt.Enable,
			secretsencrypt.Disable,
			secretsencrypt.Prepare,
			secretsencrypt.Rotate,
			secretsencrypt.Reencrypt,
			secretsencrypt.RotateKeys,
		),
	}

	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
