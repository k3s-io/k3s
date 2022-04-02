package main

import (
	"context"
	"errors"
	"os"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewCompletionCommand(),
	}

	if err := app.Run(os.Args); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
