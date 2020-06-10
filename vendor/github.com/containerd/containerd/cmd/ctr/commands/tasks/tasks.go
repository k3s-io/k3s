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

	"github.com/urfave/cli"
)

type resizer interface {
	Resize(ctx gocontext.Context, w, h uint32) error
}

// Command is the cli command for managing tasks
var Command = cli.Command{
	Name:    "tasks",
	Usage:   "manage tasks",
	Aliases: []string{"t", "task"},
	Subcommands: []cli.Command{
		attachCommand,
		checkpointCommand,
		deleteCommand,
		execCommand,
		listCommand,
		killCommand,
		pauseCommand,
		psCommand,
		resumeCommand,
		startCommand,
	},
}
