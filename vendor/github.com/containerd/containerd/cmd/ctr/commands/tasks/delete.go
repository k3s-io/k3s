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
	gocontext "context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/log"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:      "delete",
	Usage:     "delete one or more tasks",
	ArgsUsage: "CONTAINER [CONTAINER, ...]",
	Aliases:   []string{"rm"},
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "force delete task process",
		},
		cli.StringFlag{
			Name:  "exec-id",
			Usage: "process ID to kill",
		},
	},
	Action: func(context *cli.Context) error {
		var (
			execID = context.String("exec-id")
			force  = context.Bool("force")
		)
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		var opts []containerd.ProcessDeleteOpts
		if force {
			opts = append(opts, containerd.WithProcessKill)
		}
		var exitErr error
		if execID != "" {
			task, err := loadTask(ctx, client, context.Args().First())
			if err != nil {
				return err
			}
			p, err := task.LoadProcess(ctx, execID, nil)
			if err != nil {
				return err
			}
			status, err := p.Delete(ctx, opts...)
			if err != nil {
				return err
			}
			if ec := status.ExitCode(); ec != 0 {
				return cli.NewExitError("", int(ec))
			}
		} else {
			for _, target := range context.Args() {
				task, err := loadTask(ctx, client, target)
				if err != nil {
					if exitErr == nil {
						exitErr = err
					}
					log.G(ctx).WithError(err).Errorf("failed to load task from %v", target)
					continue
				}
				status, err := task.Delete(ctx, opts...)
				if err != nil {
					if exitErr == nil {
						exitErr = err
					}
					log.G(ctx).WithError(err).Errorf("unable to delete %v", task.ID())
					continue
				}
				if ec := status.ExitCode(); ec != 0 {
					log.G(ctx).Warnf("task %v exit with non-zero exit code %v", task.ID(), int(ec))
				}
			}
		}
		return exitErr
	},
}

func loadTask(ctx gocontext.Context, client *containerd.Client, containerID string) (containerd.Task, error) {
	container, err := client.LoadContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}
	task, err := container.Task(ctx, cio.Load)
	if err != nil {
		return nil, err
	}
	return task, nil
}
