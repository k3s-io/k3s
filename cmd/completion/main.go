package main

import (
	"context"
	"errors"
	"os"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/completion"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []*cli.Command{
		cmds.NewCompletionCommand(completion.Run),
	}

	if err := app.Run(context.Background(), os.Args); err != nil && !errors.Is(err, context.Canceled) {
		logrus.Fatal(err)
	}
}
