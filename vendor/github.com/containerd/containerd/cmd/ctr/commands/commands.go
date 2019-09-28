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

package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/defaults"
	"github.com/urfave/cli"
)

var (
	// SnapshotterFlags are cli flags specifying snapshotter names
	SnapshotterFlags = []cli.Flag{
		cli.StringFlag{
			Name:   "snapshotter",
			Usage:  "snapshotter name. Empty value stands for the default value.",
			EnvVar: "CONTAINERD_SNAPSHOTTER",
		},
	}

	// LabelFlag is a cli flag specifying labels
	LabelFlag = cli.StringSliceFlag{
		Name:  "label",
		Usage: "labels to attach to the image",
	}

	// RegistryFlags are cli flags specifying registry options
	RegistryFlags = []cli.Flag{
		cli.BoolFlag{
			Name:  "skip-verify,k",
			Usage: "skip SSL certificate validation",
		},
		cli.BoolFlag{
			Name:  "plain-http",
			Usage: "allow connections using plain HTTP",
		},
		cli.StringFlag{
			Name:  "user,u",
			Usage: "user[:password] Registry user and password",
		},
		cli.StringFlag{
			Name:  "refresh",
			Usage: "refresh token for authorization server",
		},
	}

	// ContainerFlags are cli flags specifying container options
	ContainerFlags = []cli.Flag{
		cli.StringFlag{
			Name:  "config,c",
			Usage: "path to the runtime-specific spec config file",
		},
		cli.StringFlag{
			Name:  "cwd",
			Usage: "specify the working directory of the process",
		},
		cli.StringSliceFlag{
			Name:  "env",
			Usage: "specify additional container environment variables (i.e. FOO=bar)",
		},
		cli.StringFlag{
			Name:  "env-file",
			Usage: "specify additional container environment variables in a file(i.e. FOO=bar, one per line)",
		},
		cli.StringSliceFlag{
			Name:  "label",
			Usage: "specify additional labels (i.e. foo=bar)",
		},
		cli.StringSliceFlag{
			Name:  "mount",
			Usage: "specify additional container mount (ex: type=bind,src=/tmp,dst=/host,options=rbind:ro)",
		},
		cli.BoolFlag{
			Name:  "net-host",
			Usage: "enable host networking for the container",
		},
		cli.BoolFlag{
			Name:  "privileged",
			Usage: "run privileged container",
		},
		cli.BoolFlag{
			Name:  "read-only",
			Usage: "set the containers filesystem as readonly",
		},
		cli.StringFlag{
			Name:  "runtime",
			Usage: "runtime name",
			Value: defaults.DefaultRuntime,
		},
		cli.BoolFlag{
			Name:  "tty,t",
			Usage: "allocate a TTY for the container",
		},
		cli.StringSliceFlag{
			Name:  "with-ns",
			Usage: "specify existing Linux namespaces to join at container runtime (format '<nstype>:<path>')",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Usage: "file path to write the task's pid",
		},
		cli.IntFlag{
			Name:  "gpus",
			Usage: "add gpus to the container",
		},
		cli.BoolFlag{
			Name:  "allow-new-privs",
			Usage: "turn off OCI spec's NoNewPrivileges feature flag",
		},
		cli.Uint64Flag{
			Name:  "memory-limit",
			Usage: "memory limit (in bytes) for the container",
		},
		cli.StringSliceFlag{
			Name:  "device",
			Usage: "add a device to a container",
		},
		cli.BoolFlag{
			Name:  "seccomp",
			Usage: "enable the default seccomp profile",
		},
	}
)

// ObjectWithLabelArgs returns the first arg and a LabelArgs object
func ObjectWithLabelArgs(clicontext *cli.Context) (string, map[string]string) {
	var (
		first        = clicontext.Args().First()
		labelStrings = clicontext.Args().Tail()
	)

	return first, LabelArgs(labelStrings)
}

// LabelArgs returns a map of label key,value pairs
func LabelArgs(labelStrings []string) map[string]string {
	labels := make(map[string]string, len(labelStrings))
	for _, label := range labelStrings {
		parts := strings.SplitN(label, "=", 2)
		key := parts[0]
		value := "true"
		if len(parts) > 1 {
			value = parts[1]
		}

		labels[key] = value
	}

	return labels
}

// PrintAsJSON prints input in JSON format
func PrintAsJSON(x interface{}) {
	b, err := json.MarshalIndent(x, "", "    ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't marshal %+v as a JSON string: %v\n", x, err)
	}
	fmt.Println(string(b))
}

// WritePidFile writes the pid atomically to a file
func WritePidFile(path string, pid int) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	tempPath := filepath.Join(filepath.Dir(path), fmt.Sprintf(".%s", filepath.Base(path)))
	f, err := os.OpenFile(tempPath, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d", pid)
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
