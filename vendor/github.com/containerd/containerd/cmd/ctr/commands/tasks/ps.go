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
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/typeurl"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var psCommand = cli.Command{
	Name:      "ps",
	Usage:     "list processes for container",
	ArgsUsage: "CONTAINER",
	Action: func(context *cli.Context) error {
		id := context.Args().First()
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
		task, err := container.Task(ctx, nil)
		if err != nil {
			return err
		}
		processes, err := task.Pids(ctx)
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 1, 8, 4, ' ', 0)
		fmt.Fprintln(w, "PID\tINFO")
		for _, ps := range processes {
			var info interface{} = "-"
			if ps.Info != nil {
				info, err = typeurl.UnmarshalAny(ps.Info)
				if err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "%d\t%+v\n", ps.Pid, info); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}
