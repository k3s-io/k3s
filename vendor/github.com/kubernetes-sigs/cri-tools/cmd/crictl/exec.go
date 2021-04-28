/*
Copyright 2017 The Kubernetes Authors.

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

package crictl

import (
	"fmt"
	"net/url"

	dockerterm "github.com/docker/docker/pkg/term"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/context"
	restclient "k8s.io/client-go/rest"
	remoteclient "k8s.io/client-go/tools/remotecommand"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/kubectl/pkg/util/term"
)

const (
	// TODO: make this configurable in kubelet.
	kubeletURLSchema = "http"
	kubeletURLHost   = "http://127.0.0.1:10250"
)

var runtimeExecCommand = &cli.Command{
	Name:                   "exec",
	Usage:                  "Run a command in a running container",
	ArgsUsage:              "CONTAINER-ID COMMAND [ARG...]",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "sync",
			Aliases: []string{"s"},
			Usage:   "Run the command synchronously",
		},
		&cli.Int64Flag{
			Name:  "timeout",
			Value: 0,
			Usage: "Timeout in seconds",
		},
		&cli.BoolFlag{
			Name:    "tty",
			Aliases: []string{"t"},
			Usage:   "Allocate a pseudo-TTY",
		},
		&cli.BoolFlag{
			Name:    "interactive",
			Aliases: []string{"i"},
			Usage:   "Keep STDIN open",
		},
	},
	Action: func(context *cli.Context) error {
		if context.Args().Len() < 2 {
			return cli.ShowSubcommandHelp(context)
		}

		runtimeClient, conn, err := getRuntimeClient(context)
		if err != nil {
			return err
		}
		defer closeConnection(context, conn)

		var opts = execOptions{
			id:      context.Args().First(),
			timeout: context.Int64("timeout"),
			tty:     context.Bool("tty"),
			stdin:   context.Bool("interactive"),
			cmd:     context.Args().Slice()[1:],
		}
		if context.Bool("sync") {
			exitCode, err := ExecSync(runtimeClient, opts)
			if err != nil {
				return errors.Wrap(err, "execing command in container synchronously")
			}
			if exitCode != 0 {
				return cli.NewExitError("non-zero exit code", exitCode)
			}
			return nil
		}
		err = Exec(runtimeClient, opts)
		if err != nil {
			return errors.Wrap(err, "execing command in container")
		}
		return nil
	},
}

// ExecSync sends an ExecSyncRequest to the server, and parses
// the returned ExecSyncResponse. The function returns the corresponding exit
// code beside an general error.
func ExecSync(client pb.RuntimeServiceClient, opts execOptions) (int, error) {
	request := &pb.ExecSyncRequest{
		ContainerId: opts.id,
		Cmd:         opts.cmd,
		Timeout:     opts.timeout,
	}
	logrus.Debugf("ExecSyncRequest: %v", request)
	r, err := client.ExecSync(context.Background(), request)
	logrus.Debugf("ExecSyncResponse: %v", r)
	if err != nil {
		return 1, err
	}
	fmt.Println(string(r.Stdout))
	fmt.Println(string(r.Stderr))
	return int(r.ExitCode), nil
}

// Exec sends an ExecRequest to server, and parses the returned ExecResponse
func Exec(client pb.RuntimeServiceClient, opts execOptions) error {
	request := &pb.ExecRequest{
		ContainerId: opts.id,
		Cmd:         opts.cmd,
		Tty:         opts.tty,
		Stdin:       opts.stdin,
		Stdout:      true,
		Stderr:      !opts.tty,
	}

	logrus.Debugf("ExecRequest: %v", request)
	r, err := client.Exec(context.Background(), request)
	logrus.Debugf("ExecResponse: %v", r)
	if err != nil {
		return err
	}
	execURL := r.Url

	URL, err := url.Parse(execURL)
	if err != nil {
		return err
	}

	if URL.Host == "" {
		URL.Host = kubeletURLHost
	}

	if URL.Scheme == "" {
		URL.Scheme = kubeletURLSchema
	}

	logrus.Debugf("Exec URL: %v", URL)
	return stream(opts.stdin, opts.tty, URL)
}

func stream(in, tty bool, url *url.URL) error {
	executor, err := remoteclient.NewSPDYExecutor(&restclient.Config{TLSClientConfig: restclient.TLSClientConfig{Insecure: true}}, "POST", url)
	if err != nil {
		return err
	}

	stdin, stdout, stderr := dockerterm.StdStreams()
	streamOptions := remoteclient.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	}
	if in {
		streamOptions.Stdin = stdin
	}
	logrus.Debugf("StreamOptions: %v", streamOptions)
	if !tty {
		return executor.Stream(streamOptions)
	}
	if !in {
		return fmt.Errorf("tty=true must be specified with interactive=true")
	}
	t := term.TTY{
		In:  stdin,
		Out: stdout,
		Raw: true,
	}
	if !t.IsTerminalIn() {
		return fmt.Errorf("input is not a terminal")
	}
	streamOptions.TerminalSizeQueue = t.MonitorSize(t.GetSize())
	return t.Safe(func() error { return executor.Stream(streamOptions) })
}
