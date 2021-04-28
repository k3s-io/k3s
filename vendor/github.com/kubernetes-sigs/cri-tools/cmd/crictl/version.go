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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/context"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

const (
	criClientVersion = "v1alpha2"
)

var runtimeVersionCommand = &cli.Command{
	Name:  "version",
	Usage: "Display runtime version information",
	Action: func(context *cli.Context) error {
		runtimeClient, runtimeConn, err := getRuntimeClient(context)
		if err != nil {
			return err
		}
		defer closeConnection(context, runtimeConn)
		err = Version(runtimeClient, criClientVersion)
		if err != nil {
			return errors.Wrap(err, "getting the runtime version")
		}
		return nil
	},
}

// Version sends a VersionRequest to the server, and parses the returned VersionResponse.
func Version(client pb.RuntimeServiceClient, version string) error {
	request := &pb.VersionRequest{Version: version}
	logrus.Debugf("VersionRequest: %v", request)
	r, err := client.Version(context.Background(), request)
	logrus.Debugf("VersionResponse: %v", r)
	if err != nil {
		return err
	}
	fmt.Println("Version: ", r.Version)
	fmt.Println("RuntimeName: ", r.RuntimeName)
	fmt.Println("RuntimeVersion: ", r.RuntimeVersion)
	fmt.Println("RuntimeApiVersion: ", r.RuntimeApiVersion)
	return nil
}
