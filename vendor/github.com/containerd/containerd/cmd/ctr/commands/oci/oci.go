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

package oci

import (
	"github.com/pkg/errors"
	"github.com/urfave/cli"

	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
)

// Command is the parent for all OCI related tools under 'oci'
var Command = cli.Command{
	Name:  "oci",
	Usage: "OCI tools",
	Subcommands: []cli.Command{
		defaultSpecCommand,
	},
}

var defaultSpecCommand = cli.Command{
	Name:  "spec",
	Usage: "see the output of the default OCI spec",
	Action: func(context *cli.Context) error {
		ctx, cancel := commands.AppContext(context)
		defer cancel()

		spec, err := oci.GenerateSpec(ctx, nil, &containers.Container{})
		if err != nil {
			return errors.Wrap(err, "failed to generate spec")
		}

		commands.PrintAsJSON(spec)
		return nil
	},
}
