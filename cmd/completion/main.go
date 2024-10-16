package main

import (
	"context"
	"errors"
	"os"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/completion"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2" // Ensure you're using the correct version
)

func main() {
	app := cmds.NewApp()
	app.Commands = []*cli.Command{
		cmds.NewCompletionCommand(completion.Run),
	}

	if err := app.Run(os.Args); err != nil {
		if errors.Is(err, context.Canceled) {
			logrus.Info("Execution canceled")
		} else {
			logrus.Fatal(err)
		}
	}
}
