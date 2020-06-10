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

package ctr

import (
	"fmt"
	"os"

	"github.com/containerd/containerd/cmd/ctr/app"
	"github.com/containerd/containerd/pkg/seed"
	"github.com/urfave/cli"
)

func Main() {
	main()
}

func main() {
	seed.WithTimeAndRand()
	app := app.New()
	for i, flag := range app.Flags {
		if sFlag, ok := flag.(cli.StringFlag); ok {
			if sFlag.Name == "address, a" {
				sFlag.Value = "/run/k3s/containerd/containerd.sock"
				app.Flags[i] = sFlag
			} else if sFlag.Name == "namespace, n" {
				sFlag.Value = "k8s.io"
				app.Flags[i] = sFlag
			}
		}
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ctr: %s\n", err)
		os.Exit(1)
	}
}
