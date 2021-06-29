// +build !windows

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
	"net/url"
	"os"
	"os/signal"

	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

func init() {
	startCommand.Flags = append(startCommand.Flags, cli.BoolFlag{
		Name:  "no-pivot",
		Usage: "disable use of pivot-root (linux only)",
	})
}

// HandleConsoleResize resizes the console
func HandleConsoleResize(ctx gocontext.Context, task resizer, con console.Console) error {
	// do an initial resize of the console
	size, err := con.Size()
	if err != nil {
		return err
	}
	if err := task.Resize(ctx, uint32(size.Width), uint32(size.Height)); err != nil {
		log.G(ctx).WithError(err).Error("resize pty")
	}
	s := make(chan os.Signal, 16)
	signal.Notify(s, unix.SIGWINCH)
	go func() {
		for range s {
			size, err := con.Size()
			if err != nil {
				log.G(ctx).WithError(err).Error("get pty size")
				continue
			}
			if err := task.Resize(ctx, uint32(size.Width), uint32(size.Height)); err != nil {
				log.G(ctx).WithError(err).Error("resize pty")
			}
		}
	}()
	return nil
}

// NewTask creates a new task
func NewTask(ctx gocontext.Context, client *containerd.Client, container containerd.Container, checkpoint string, con console.Console, nullIO bool, logURI string, ioOpts []cio.Opt, opts ...containerd.NewTaskOpts) (containerd.Task, error) {
	stdinC := &stdinCloser{
		stdin: os.Stdin,
	}
	stdio := cio.NewCreator(append([]cio.Opt{cio.WithStreams(stdinC, os.Stdout, os.Stderr)}, ioOpts...)...)
	if checkpoint != "" {
		im, err := client.GetImage(ctx, checkpoint)
		if err != nil {
			return nil, err
		}
		opts = append(opts, containerd.WithTaskCheckpoint(im))
	}
	ioCreator := stdio
	if con != nil {
		ioCreator = cio.NewCreator(append([]cio.Opt{cio.WithStreams(con, con, nil), cio.WithTerminal}, ioOpts...)...)
	}
	if nullIO {
		if con != nil {
			return nil, errors.New("tty and null-io cannot be used together")
		}
		ioCreator = cio.NullIO
	}
	if logURI != "" {
		u, err := url.Parse(logURI)
		if err != nil {
			return nil, err
		}
		ioCreator = cio.LogURI(u)
	}
	t, err := container.NewTask(ctx, ioCreator, opts...)
	if err != nil {
		return nil, err
	}
	stdinC.closer = func() {
		t.CloseIO(ctx, containerd.WithStdinCloser)
	}
	return t, nil
}

func getNewTaskOpts(context *cli.Context) []containerd.NewTaskOpts {
	if context.Bool("no-pivot") {
		return []containerd.NewTaskOpts{containerd.WithNoPivotRoot}
	}
	return nil
}
