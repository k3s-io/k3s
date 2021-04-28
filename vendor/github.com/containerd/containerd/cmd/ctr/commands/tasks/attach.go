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

package tasks

import (
	"github.com/containerd/console"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var attachCommand = cli.Command{
	Name:      "attach",
	Usage:     "attach to the IO of a running container",
	ArgsUsage: "CONTAINER",
	Action: func(context *cli.Context) error {
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		container, err := client.LoadContainer(ctx, context.Args().First())
		if err != nil {
			return err
		}
		spec, err := container.Spec(ctx)
		if err != nil {
			return err
		}
		var (
			con console.Console
			tty = spec.Process.Terminal
		)
		if tty {
			con = console.Current()
			defer con.Reset()
			if err := con.SetRaw(); err != nil {
				return err
			}
		}
		task, err := container.Task(ctx, cio.NewAttach(cio.WithStdio))
		if err != nil {
			return err
		}
		defer task.Delete(ctx)

		statusC, err := task.Wait(ctx)
		if err != nil {
			return err
		}

		if tty {
			if err := HandleConsoleResize(ctx, task, con); err != nil {
				logrus.WithError(err).Error("console resize")
			}
		} else {
			sigc := commands.ForwardAllSignals(ctx, task)
			defer commands.StopCatch(sigc)
		}

		ec := <-statusC
		code, _, err := ec.Result()
		if err != nil {
			return err
		}
		if code != 0 {
			return cli.NewExitError("", int(code))
		}
		return nil
	},
}
