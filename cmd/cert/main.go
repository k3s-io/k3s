package main

import (
	"context"
	"errors"
	"os"

	"github.com/k3s-io/k3s/pkg/cli/cert"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewCertCommands(
			cert.Check,
			cert.Rotate,
			cert.RotateCA,
		),
	}

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the application and handle errors more effectively
	if err := app.Run(configfilearg.MustParse(os.Args)); err != nil {
		if errors.Is(err, context.Canceled) {
			logrus.Info("Application canceled")
		} else {
			logrus.WithError(err).Fatal("Failed to run application")
		}
	}
}
