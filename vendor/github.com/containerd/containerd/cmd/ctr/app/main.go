/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package app

import (
	"fmt"
	"io/ioutil"

	"github.com/containerd/containerd/cmd/ctr/commands/containers"
	"github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/cmd/ctr/commands/events"
	"github.com/containerd/containerd/cmd/ctr/commands/images"
	"github.com/containerd/containerd/cmd/ctr/commands/install"
	"github.com/containerd/containerd/cmd/ctr/commands/leases"
	namespacesCmd "github.com/containerd/containerd/cmd/ctr/commands/namespaces"
	"github.com/containerd/containerd/cmd/ctr/commands/plugins"
	"github.com/containerd/containerd/cmd/ctr/commands/pprof"
	"github.com/containerd/containerd/cmd/ctr/commands/run"
	"github.com/containerd/containerd/cmd/ctr/commands/snapshots"
	"github.com/containerd/containerd/cmd/ctr/commands/tasks"
	versionCmd "github.com/containerd/containerd/cmd/ctr/commands/version"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"google.golang.org/grpc/grpclog"
)

var extraCmds = []cli.Command{}

func init() {
	// Discard grpc logs so that they don't mess with our stdio
	grpclog.SetLoggerV2(grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, ioutil.Discard))

	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Println(c.App.Name, version.Package, c.App.Version)
	}
}

// New returns a *cli.App instance.
func New() *cli.App {
	app := cli.NewApp()
	app.Name = "ctr"
	app.Version = version.Version
	app.Usage = `
        __
  _____/ /______
 / ___/ __/ ___/
/ /__/ /_/ /
\___/\__/_/

containerd CLI
`
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output in logs",
		},
		cli.StringFlag{
			Name:  "address, a",
			Usage: "address for containerd's GRPC server",
			Value: defaults.DefaultAddress,
		},
		cli.DurationFlag{
			Name:  "timeout",
			Usage: "total timeout for ctr commands",
		},
		cli.DurationFlag{
			Name:  "connect-timeout",
			Usage: "timeout for connecting to containerd",
		},
		cli.StringFlag{
			Name:   "namespace, n",
			Usage:  "namespace to use with commands",
			Value:  namespaces.Default,
			EnvVar: namespaces.NamespaceEnvVar,
		},
	}
	app.Commands = append([]cli.Command{
		plugins.Command,
		versionCmd.Command,
		containers.Command,
		content.Command,
		events.Command,
		images.Command,
		leases.Command,
		namespacesCmd.Command,
		pprof.Command,
		run.Command,
		snapshots.Command,
		tasks.Command,
		install.Command,
	}, extraCmds...)
	app.Before = func(context *cli.Context) error {
		if context.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	return app
}
