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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/context"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

var runtimeAttachCommand = &cli.Command{
	Name:                   "attach",
	Usage:                  "Attach to a running container",
	ArgsUsage:              "CONTAINER-ID",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "tty",
			Aliases: []string{"t"},
			Usage:   "Allocate a pseudo-TTY",
		},
		&cli.BoolFlag{
			Name:    "stdin",
			Aliases: []string{"i"},
			Usage:   "Keep STDIN open",
		},
	},
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		if id == "" {
			return cli.ShowSubcommandHelp(context)
		}

		runtimeClient, conn, err := getRuntimeClient(context)
		if err != nil {
			return err
		}
		defer closeConnection(context, conn)

		var opts = attachOptions{
			id:    id,
			tty:   context.Bool("tty"),
			stdin: context.Bool("stdin"),
		}
		err = Attach(runtimeClient, opts)
		if err != nil {
			return errors.Wrap(err, "attaching running container failed")

		}
		return nil
	},
}

// Attach sends an AttachRequest to server, and parses the returned AttachResponse
func Attach(client pb.RuntimeServiceClient, opts attachOptions) error {
	if opts.id == "" {
		return fmt.Errorf("ID cannot be empty")

	}
	request := &pb.AttachRequest{
		ContainerId: opts.id,
		Tty:         opts.tty,
		Stdin:       opts.stdin,
		Stdout:      true,
		Stderr:      !opts.tty,
	}
	logrus.Debugf("AttachRequest: %v", request)
	r, err := client.Attach(context.Background(), request)
	logrus.Debugf("AttachResponse: %v", r)
	if err != nil {
		return err
	}
	attachURL := r.Url

	URL, err := url.Parse(attachURL)
	if err != nil {
		return err
	}

	if URL.Host == "" {
		URL.Host = kubeletURLHost
	}
	if URL.Scheme == "" {
		URL.Scheme = kubeletURLSchema
	}

	logrus.Debugf("Attach URL: %v", URL)
	return stream(opts.stdin, opts.tty, URL)
}
