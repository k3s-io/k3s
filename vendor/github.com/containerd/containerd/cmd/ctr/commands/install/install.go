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

package install

import (
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/urfave/cli"
)

// Command to install binary packages
var Command = cli.Command{
	Name:        "install",
	Usage:       "install a new package",
	ArgsUsage:   "<ref>",
	Description: "install a new package",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "libs,l",
			Usage: "install libs from the image",
		},
		cli.BoolFlag{
			Name:  "replace,r",
			Usage: "replace any binaries or libs in the opt directory",
		},
		cli.StringFlag{
			Name:  "path",
			Usage: "set an optional install path other than the managed opt directory",
		},
	},
	Action: func(context *cli.Context) error {
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		ref := context.Args().First()
		image, err := client.GetImage(ctx, ref)
		if err != nil {
			return err
		}
		var opts []containerd.InstallOpts
		if context.Bool("libs") {
			opts = append(opts, containerd.WithInstallLibs)
		}
		if context.Bool("replace") {
			opts = append(opts, containerd.WithInstallReplace)
		}
		if path := context.String("path"); path != "" {
			opts = append(opts, containerd.WithInstallPath(path))
		}
		return client.Install(ctx, image, opts...)
	},
}
