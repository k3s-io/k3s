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
	"errors"

	"github.com/containerd/console"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

//TODO:(jessvalarezo) exec-id is optional here, update to required arg
var execCommand = cli.Command{
	Name:      "exec",
	Usage:     "execute additional processes in an existing container",
	ArgsUsage: "[flags] CONTAINER CMD [ARG...]",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "cwd",
			Usage: "working directory of the new process",
		},
		cli.BoolFlag{
			Name:  "tty,t",
			Usage: "allocate a TTY for the container",
		},
		cli.StringFlag{
			Name:  "exec-id",
			Usage: "exec specific id for the process",
		},
		cli.StringFlag{
			Name:  "fifo-dir",
			Usage: "directory used for storing IO FIFOs",
		},
	},
	Action: func(context *cli.Context) error {
		var (
			id   = context.Args().First()
			args = context.Args().Tail()
			tty  = context.Bool("tty")
		)
		if id == "" {
			return errors.New("container id must be provided")
		}
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		spec, err := container.Spec(ctx)
		if err != nil {
			return err
		}
		task, err := container.Task(ctx, nil)
		if err != nil {
			return err
		}

		pspec := spec.Process
		pspec.Terminal = tty
		pspec.Args = args

		cioOpts := []cio.Opt{cio.WithStdio, cio.WithFIFODir(context.String("fifo-dir"))}
		if tty {
			cioOpts = append(cioOpts, cio.WithTerminal)
		}
		ioCreator := cio.NewCreator(cioOpts...)
		process, err := task.Exec(ctx, context.String("exec-id"), pspec, ioCreator)
		if err != nil {
			return err
		}
		defer process.Delete(ctx)

		statusC, err := process.Wait(ctx)
		if err != nil {
			return err
		}

		var con console.Console
		if tty {
			con = console.Current()
			defer con.Reset()
			if err := con.SetRaw(); err != nil {
				return err
			}
		}
		if tty {
			if err := HandleConsoleResize(ctx, process, con); err != nil {
				logrus.WithError(err).Error("console resize")
			}
		} else {
			sigc := commands.ForwardAllSignals(ctx, process)
			defer commands.StopCatch(sigc)
		}

		if err := process.Start(ctx); err != nil {
			return err
		}
		status := <-statusC
		code, _, err := status.Result()
		if err != nil {
			return err
		}
		if code != 0 {
			return cli.NewExitError("", int(code))
		}
		return nil
	},
}
