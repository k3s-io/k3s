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
	"time"

	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// HandleConsoleResize resizes the console
func HandleConsoleResize(ctx gocontext.Context, task resizer, con console.Console) error {
	// do an initial resize of the console
	size, err := con.Size()
	if err != nil {
		return err
	}
	go func() {
		prevSize := size
		for {
			time.Sleep(time.Millisecond * 250)

			size, err := con.Size()
			if err != nil {
				log.G(ctx).WithError(err).Error("get pty size")
				continue
			}

			if size.Width != prevSize.Width || size.Height != prevSize.Height {
				if err := task.Resize(ctx, uint32(size.Width), uint32(size.Height)); err != nil {
					log.G(ctx).WithError(err).Error("resize pty")
				}
				prevSize = size
			}
		}
	}()
	return nil
}

// NewTask creates a new task
func NewTask(ctx gocontext.Context, client *containerd.Client, container containerd.Container, _ string, con console.Console, nullIO bool, logURI string, ioOpts []cio.Opt, opts ...containerd.NewTaskOpts) (containerd.Task, error) {
	var ioCreator cio.Creator
	if con != nil {
		if nullIO {
			return nil, errors.New("tty and null-io cannot be used together")
		}
		ioCreator = cio.NewCreator(append([]cio.Opt{cio.WithStreams(con, con, nil), cio.WithTerminal}, ioOpts...)...)
	} else if nullIO {
		ioCreator = cio.NullIO
	} else {
		ioCreator = cio.NewCreator(append([]cio.Opt{cio.WithStdio}, ioOpts...)...)
	}
	return container.NewTask(ctx, ioCreator)
}

func getNewTaskOpts(_ *cli.Context) []containerd.NewTaskOpts {
	return nil
}
