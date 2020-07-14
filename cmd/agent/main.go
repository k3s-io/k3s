package main

import (
	"os"

	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
)

func main() {
	app := cmds.NewApp()
	app.Commands = []cli.Command{
		cmds.NewAgentCommand(agent.Run),
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}
