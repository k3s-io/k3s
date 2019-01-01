//go:generate go run types/codegen/cleanup/main.go
//go:generate go run types/codegen/main.go

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/reexec"
	"github.com/rancher/rio/cli/cmd/agent"
	"github.com/rancher/rio/cli/cmd/kubectl"
	"github.com/rancher/rio/cli/cmd/server"
	"github.com/rancher/rio/cli/pkg/builder"
	"github.com/rancher/rio/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	_ "github.com/rancher/rio/pkg/kubectl"
)

var (
	appName = filepath.Base(os.Args[0])
	debug   bool
)

func main() {
	old := os.Args[0]
	os.Args[0] = filepath.Base(os.Args[0])
	if reexec.Init() {
		return
	}
	os.Args[0] = old

	app := cli.NewApp()
	app.Name = appName
	app.Usage = "Kubernetes, but small and simple"
	app.Version = version.Version
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Turn on debug logs",
			Destination: &debug,
		},
	}

	app.Commands = []cli.Command{
		server.ServerCommand,
		builder.Command(&agent.Agent{},
			"Run node agent",
			appName+" agent [OPTIONS]",
			""),

		kubectl.NewKubectlCommand(),
	}
	app.Before = func(ctx *cli.Context) error {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}
