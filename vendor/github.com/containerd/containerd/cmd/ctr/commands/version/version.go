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

package version

import (
	"fmt"
	"os"

	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/version"
	"github.com/urfave/cli"
)

// Command is a cli command to output the client and containerd server version
var Command = cli.Command{
	Name:  "version",
	Usage: "print the client and server versions",
	Action: func(context *cli.Context) error {
		fmt.Println("Client:")
		fmt.Println("  Version: ", version.Version)
		fmt.Println("  Revision:", version.Revision)
		fmt.Println("")
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		v, err := client.Version(ctx)
		if err != nil {
			return err
		}
		fmt.Println("Server:")
		fmt.Println("  Version: ", v.Version)
		fmt.Println("  Revision:", v.Revision)
		if v.Version != version.Version {
			fmt.Fprintln(os.Stderr, "WARNING: version mismatch")
		}
		if v.Revision != version.Revision {
			fmt.Fprintln(os.Stderr, "WARNING: revision mismatch")
		}
		return nil
	},
}
