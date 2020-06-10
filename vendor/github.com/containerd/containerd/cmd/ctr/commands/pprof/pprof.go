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

package pprof

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/containerd/containerd/defaults"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

type pprofDialer struct {
	proto string
	addr  string
}

// Command is the cli command for providing golang pprof outputs for containerd
var Command = cli.Command{
	Name:  "pprof",
	Usage: "provide golang pprof outputs for containerd",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "debug-socket, d",
			Usage: "socket path for containerd's debug server",
			Value: defaults.DefaultDebugAddress,
		},
	},
	Subcommands: []cli.Command{
		pprofBlockCommand,
		pprofGoroutinesCommand,
		pprofHeapCommand,
		pprofProfileCommand,
		pprofThreadcreateCommand,
		pprofTraceCommand,
	},
}

var pprofGoroutinesCommand = cli.Command{
	Name:  "goroutines",
	Usage: "dump goroutine stack dump",
	Action: func(context *cli.Context) error {
		client := getPProfClient(context)

		output, err := httpGetRequest(client, "/debug/pprof/goroutine?debug=2")
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(os.Stdout, output)
		return err
	},
}

var pprofHeapCommand = cli.Command{
	Name:  "heap",
	Usage: "dump heap profile",
	Action: func(context *cli.Context) error {
		client := getPProfClient(context)

		output, err := httpGetRequest(client, "/debug/pprof/heap")
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(os.Stdout, output)
		return err
	},
}

var pprofProfileCommand = cli.Command{
	Name:  "profile",
	Usage: "CPU profile",
	Flags: []cli.Flag{
		cli.DurationFlag{
			Name:  "seconds,s",
			Usage: "duration for collection (seconds)",
			Value: 30 * time.Second,
		},
	},
	Action: func(context *cli.Context) error {
		client := getPProfClient(context)

		seconds := context.Duration("seconds").Seconds()
		output, err := httpGetRequest(client, fmt.Sprintf("/debug/pprof/profile?seconds=%v", seconds))
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(os.Stdout, output)
		return err
	},
}

var pprofTraceCommand = cli.Command{
	Name:  "trace",
	Usage: "collect execution trace",
	Flags: []cli.Flag{
		cli.DurationFlag{
			Name:  "seconds,s",
			Usage: "trace time (seconds)",
			Value: 5 * time.Second,
		},
	},
	Action: func(context *cli.Context) error {
		client := getPProfClient(context)

		seconds := context.Duration("seconds").Seconds()
		uri := fmt.Sprintf("/debug/pprof/trace?seconds=%v", seconds)
		output, err := httpGetRequest(client, uri)
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(os.Stdout, output)
		return err
	},
}

var pprofBlockCommand = cli.Command{
	Name:  "block",
	Usage: "goroutine blocking profile",
	Action: func(context *cli.Context) error {
		client := getPProfClient(context)

		output, err := httpGetRequest(client, "/debug/pprof/block")
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(os.Stdout, output)
		return err
	},
}

var pprofThreadcreateCommand = cli.Command{
	Name:  "threadcreate",
	Usage: "goroutine thread creating profile",
	Action: func(context *cli.Context) error {
		client := getPProfClient(context)

		output, err := httpGetRequest(client, "/debug/pprof/threadcreate")
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(os.Stdout, output)
		return err
	},
}

func getPProfClient(context *cli.Context) *http.Client {
	dialer := getPProfDialer(context.GlobalString("debug-socket"))

	tr := &http.Transport{
		Dial: dialer.pprofDial,
	}
	client := &http.Client{Transport: tr}
	return client
}

func httpGetRequest(client *http.Client, request string) (io.ReadCloser, error) {
	resp, err := client.Get("http://." + request)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, errors.Errorf("http get failed with status: %s", resp.Status)
	}
	return resp.Body, nil
}
