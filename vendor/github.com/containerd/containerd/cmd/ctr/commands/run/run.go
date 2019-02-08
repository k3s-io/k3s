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

package run

import (
	gocontext "context"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/cmd/ctr/commands/tasks"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func withMounts(context *cli.Context) oci.SpecOpts {
	return func(ctx gocontext.Context, client oci.Client, container *containers.Container, s *specs.Spec) error {
		mounts := make([]specs.Mount, 0)
		for _, mount := range context.StringSlice("mount") {
			m, err := parseMountFlag(mount)
			if err != nil {
				return err
			}
			mounts = append(mounts, m)
		}
		return oci.WithMounts(mounts)(ctx, client, container, s)
	}
}

// parseMountFlag parses a mount string in the form "type=foo,source=/path,destination=/target,options=rbind:rw"
func parseMountFlag(m string) (specs.Mount, error) {
	mount := specs.Mount{}
	r := csv.NewReader(strings.NewReader(m))

	fields, err := r.Read()
	if err != nil {
		return mount, err
	}

	for _, field := range fields {
		v := strings.Split(field, "=")
		if len(v) != 2 {
			return mount, fmt.Errorf("invalid mount specification: expected key=val")
		}

		key := v[0]
		val := v[1]
		switch key {
		case "type":
			mount.Type = val
		case "source", "src":
			mount.Source = val
		case "destination", "dst":
			mount.Destination = val
		case "options":
			mount.Options = strings.Split(val, ":")
		default:
			return mount, fmt.Errorf("mount option %q not supported", key)
		}
	}

	return mount, nil
}

// Command runs a container
var Command = cli.Command{
	Name:      "run",
	Usage:     "run a container",
	ArgsUsage: "[flags] Image|RootFS ID [COMMAND] [ARG...]",
	Flags: append([]cli.Flag{
		cli.BoolFlag{
			Name:  "rm",
			Usage: "remove the container after running",
		},
		cli.BoolFlag{
			Name:  "null-io",
			Usage: "send all IO to /dev/null",
		},
		cli.BoolFlag{
			Name:  "detach,d",
			Usage: "detach from the task after it has started execution",
		},
		cli.StringFlag{
			Name:  "fifo-dir",
			Usage: "directory used for storing IO FIFOs",
		},
		cli.BoolFlag{
			Name:  "isolated",
			Usage: "run the container with vm isolation",
		},
	}, append(commands.SnapshotterFlags, commands.ContainerFlags...)...),
	Action: func(context *cli.Context) error {
		var (
			err error
			id  string
			ref string

			tty    = context.Bool("tty")
			detach = context.Bool("detach")
			config = context.IsSet("config")
		)

		if config {
			id = context.Args().First()
			if context.NArg() > 1 {
				return errors.New("with spec config file, only container id should be provided")
			}
		} else {
			id = context.Args().Get(1)
			ref = context.Args().First()

			if ref == "" {
				return errors.New("image ref must be provided")
			}
		}
		if id == "" {
			return errors.New("container id must be provided")
		}
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		container, err := NewContainer(ctx, client, context)
		if err != nil {
			return err
		}
		if context.Bool("rm") && !detach {
			defer container.Delete(ctx, containerd.WithSnapshotCleanup)
		}
		var con console.Console
		if tty {
			con = console.Current()
			defer con.Reset()
			if err := con.SetRaw(); err != nil {
				return err
			}
		}
		opts := getNewTaskOpts(context)
		ioOpts := []cio.Opt{cio.WithFIFODir(context.String("fifo-dir"))}
		task, err := tasks.NewTask(ctx, client, container, context.String("checkpoint"), con, context.Bool("null-io"), ioOpts, opts...)
		if err != nil {
			return err
		}
		var statusC <-chan containerd.ExitStatus
		if !detach {
			defer task.Delete(ctx)
			if statusC, err = task.Wait(ctx); err != nil {
				return err
			}
		}
		if context.IsSet("pid-file") {
			if err := commands.WritePidFile(context.String("pid-file"), int(task.Pid())); err != nil {
				return err
			}
		}
		if err := task.Start(ctx); err != nil {
			return err
		}
		if detach {
			return nil
		}
		if tty {
			if err := tasks.HandleConsoleResize(ctx, task, con); err != nil {
				logrus.WithError(err).Error("console resize")
			}
		} else {
			sigc := commands.ForwardAllSignals(ctx, task)
			defer commands.StopCatch(sigc)
		}
		status := <-statusC
		code, _, err := status.Result()
		if err != nil {
			return err
		}
		if _, err := task.Delete(ctx); err != nil {
			return err
		}
		if code != 0 {
			return cli.NewExitError("", int(code))
		}
		return nil
	},
}
