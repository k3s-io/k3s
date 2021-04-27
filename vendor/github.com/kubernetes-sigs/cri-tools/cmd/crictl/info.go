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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/context"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

var runtimeStatusCommand = &cli.Command{
	Name:                   "info",
	Usage:                  "Display information of the container runtime",
	ArgsUsage:              "",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Value:   "json",
			Usage:   "Output format, One of: json|yaml|go-template",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "Do not show verbose information",
		},
		&cli.StringFlag{
			Name:  "template",
			Usage: "The template string is only used when output is go-template; The Template format is golang template",
		},
	},
	Action: func(context *cli.Context) error {
		runtimeClient, runtimeConn, err := getRuntimeClient(context)
		if err != nil {
			return err
		}
		defer closeConnection(context, runtimeConn)

		err = Info(context, runtimeClient)
		if err != nil {
			return errors.Wrap(err, "getting status of runtime")
		}
		return nil
	},
}

// Info sends a StatusRequest to the server, and parses the returned StatusResponse.
func Info(cliContext *cli.Context, client pb.RuntimeServiceClient) error {
	request := &pb.StatusRequest{Verbose: !cliContext.Bool("quiet")}
	logrus.Debugf("StatusRequest: %v", request)
	r, err := client.Status(context.Background(), request)
	logrus.Debugf("StatusResponse: %v", r)
	if err != nil {
		return err
	}

	status, err := protobufObjectToJSON(r.Status)
	if err != nil {
		return err
	}
	return outputStatusInfo(status, r.Info, cliContext.String("output"), cliContext.String("template"))
}
