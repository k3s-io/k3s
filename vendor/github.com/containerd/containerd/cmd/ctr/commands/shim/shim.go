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

package shim

import (
	gocontext "context"
	"fmt"
	"io/ioutil"
	"net"

	"github.com/containerd/console"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var fifoFlags = []cli.Flag{
	cli.StringFlag{
		Name:  "stdin",
		Usage: "specify the path to the stdin fifo",
	},
	cli.StringFlag{
		Name:  "stdout",
		Usage: "specify the path to the stdout fifo",
	},
	cli.StringFlag{
		Name:  "stderr",
		Usage: "specify the path to the stderr fifo",
	},
	cli.BoolFlag{
		Name:  "tty,t",
		Usage: "enable tty support",
	},
}

// Command is the cli command for interacting with a task
var Command = cli.Command{
	Name:  "shim",
	Usage: "interact with a shim directly",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "socket",
			Usage: "socket on which to connect to the shim",
		},
	},
	Subcommands: []cli.Command{
		deleteCommand,
		execCommand,
		startCommand,
		stateCommand,
	},
}

var startCommand = cli.Command{
	Name:  "start",
	Usage: "start a container with a task",
	Action: func(context *cli.Context) error {
		service, err := getTaskService(context)
		if err != nil {
			return err
		}
		_, err = service.Start(gocontext.Background(), &task.StartRequest{
			ID: context.Args().First(),
		})
		return err
	},
}

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete a container with a task",
	Action: func(context *cli.Context) error {
		service, err := getTaskService(context)
		if err != nil {
			return err
		}
		r, err := service.Delete(gocontext.Background(), &task.DeleteRequest{
			ID: context.Args().First(),
		})
		if err != nil {
			return err
		}
		fmt.Printf("container deleted and returned exit status %d\n", r.ExitStatus)
		return nil
	},
}

var stateCommand = cli.Command{
	Name:  "state",
	Usage: "get the state of all the processes of the task",
	Action: func(context *cli.Context) error {
		service, err := getTaskService(context)
		if err != nil {
			return err
		}
		r, err := service.State(gocontext.Background(), &task.StateRequest{
			ID: context.Args().First(),
		})
		if err != nil {
			return err
		}
		commands.PrintAsJSON(r)
		return nil
	},
}

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "exec a new process in the task's container",
	Flags: append(fifoFlags,
		cli.BoolFlag{
			Name:  "attach,a",
			Usage: "stay attached to the container and open the fifos",
		},
		cli.StringSliceFlag{
			Name:  "env,e",
			Usage: "add environment vars",
			Value: &cli.StringSlice{},
		},
		cli.StringFlag{
			Name:  "cwd",
			Usage: "current working directory",
		},
		cli.StringFlag{
			Name:  "spec",
			Usage: "runtime spec",
		},
	),
	Action: func(context *cli.Context) error {
		service, err := getTaskService(context)
		if err != nil {
			return err
		}
		var (
			id  = context.Args().First()
			ctx = gocontext.Background()
		)

		if id == "" {
			return errors.New("exec id must be provided")
		}

		tty := context.Bool("tty")
		wg, err := prepareStdio(context.String("stdin"), context.String("stdout"), context.String("stderr"), tty)
		if err != nil {
			return err
		}

		// read spec file and extract Any object
		spec, err := ioutil.ReadFile(context.String("spec"))
		if err != nil {
			return err
		}
		url, err := typeurl.TypeURL(specs.Process{})
		if err != nil {
			return err
		}

		rq := &task.ExecProcessRequest{
			ID: id,
			Spec: &ptypes.Any{
				TypeUrl: url,
				Value:   spec,
			},
			Stdin:    context.String("stdin"),
			Stdout:   context.String("stdout"),
			Stderr:   context.String("stderr"),
			Terminal: tty,
		}
		if _, err := service.Exec(ctx, rq); err != nil {
			return err
		}
		r, err := service.Start(ctx, &task.StartRequest{
			ID: id,
		})
		if err != nil {
			return err
		}
		fmt.Printf("exec running with pid %d\n", r.Pid)
		if context.Bool("attach") {
			logrus.Info("attaching")
			if tty {
				current := console.Current()
				defer current.Reset()
				if err := current.SetRaw(); err != nil {
					return err
				}
				size, err := current.Size()
				if err != nil {
					return err
				}
				if _, err := service.ResizePty(ctx, &task.ResizePtyRequest{
					ID:     id,
					Width:  uint32(size.Width),
					Height: uint32(size.Height),
				}); err != nil {
					return err
				}
			}
			wg.Wait()
		}
		return nil
	},
}

func getTaskService(context *cli.Context) (task.TaskService, error) {
	bindSocket := context.GlobalString("socket")
	if bindSocket == "" {
		return nil, errors.New("socket path must be specified")
	}

	conn, err := net.Dial("unix", "\x00"+bindSocket)
	if err != nil {
		return nil, err
	}

	client := ttrpc.NewClient(conn)

	// TODO(stevvooe): This actually leaks the connection. We were leaking it
	// before, so may not be a huge deal.

	return task.NewTaskClient(client), nil
}
