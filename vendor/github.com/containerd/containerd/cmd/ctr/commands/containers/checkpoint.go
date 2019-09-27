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

package containers

import (
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var checkpointCommand = cli.Command{
	Name:      "checkpoint",
	Usage:     "checkpoint a container",
	ArgsUsage: "CONTAINER REF",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "rw",
			Usage: "include the rw layer in the checkpoint",
		},
		cli.BoolFlag{
			Name:  "image",
			Usage: "include the image in the checkpoint",
		},
		cli.BoolFlag{
			Name:  "task",
			Usage: "checkpoint container task",
		},
	},
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return errors.New("container id must be provided")
		}
		ref := context.Args().Get(1)
		if ref == "" {
			return errors.New("ref must be provided")
		}
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		opts := []containerd.CheckpointOpts{
			containerd.WithCheckpointRuntime,
		}

		if context.Bool("image") {
			opts = append(opts, containerd.WithCheckpointImage)
		}
		if context.Bool("rw") {
			opts = append(opts, containerd.WithCheckpointRW)
		}
		if context.Bool("task") {
			opts = append(opts, containerd.WithCheckpointTask)
		}
		container, err := client.LoadContainer(ctx, id)
		if err != nil {
			return err
		}
		task, err := container.Task(ctx, nil)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return err
			}
		}
		// pause if running
		if task != nil {
			if err := task.Pause(ctx); err != nil {
				return err
			}
			defer func() {
				if err := task.Resume(ctx); err != nil {
					fmt.Println(errors.Wrap(err, "error resuming task"))
				}
			}()
		}

		if _, err := container.Checkpoint(ctx, ref, opts...); err != nil {
			return err
		}

		return nil
	},
}
