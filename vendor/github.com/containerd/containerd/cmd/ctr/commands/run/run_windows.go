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

package run

import (
	gocontext "context"

	"github.com/Microsoft/hcsshim/cmd/containerd-shim-runhcs-v1/options"
	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var platformRunFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "isolated",
		Usage: "run the container with vm isolation",
	},
}

// NewContainer creates a new container
func NewContainer(ctx gocontext.Context, client *containerd.Client, context *cli.Context) (containerd.Container, error) {
	var (
		id    string
		opts  []oci.SpecOpts
		cOpts []containerd.NewContainerOpts
		spec  containerd.NewContainerOpts

		config = context.IsSet("config")
	)

	if config {
		id = context.Args().First()
		opts = append(opts, oci.WithSpecFromFile(context.String("config")))
		cOpts = append(cOpts, containerd.WithContainerLabels(commands.LabelArgs(context.StringSlice("label"))))
	} else {
		var (
			ref  = context.Args().First()
			args = context.Args()[2:]
		)

		id = context.Args().Get(1)
		snapshotter := context.String("snapshotter")
		if snapshotter == "windows-lcow" {
			opts = append(opts, oci.WithDefaultSpecForPlatform("linux/amd64"))
			// Clear the rootfs section.
			opts = append(opts, oci.WithRootFSPath(""))
		} else {
			opts = append(opts, oci.WithDefaultSpec())
			opts = append(opts, oci.WithWindowNetworksAllowUnqualifiedDNSQuery())
			opts = append(opts, oci.WithWindowsIgnoreFlushesDuringBoot())
		}
		if ef := context.String("env-file"); ef != "" {
			opts = append(opts, oci.WithEnvFile(ef))
		}
		opts = append(opts, oci.WithEnv(context.StringSlice("env")))
		opts = append(opts, withMounts(context))

		image, err := client.GetImage(ctx, ref)
		if err != nil {
			return nil, err
		}
		unpacked, err := image.IsUnpacked(ctx, snapshotter)
		if err != nil {
			return nil, err
		}
		if !unpacked {
			if err := image.Unpack(ctx, snapshotter); err != nil {
				return nil, err
			}
		}
		opts = append(opts, oci.WithImageConfig(image))
		labels := buildLabels(commands.LabelArgs(context.StringSlice("label")), image.Labels())
		cOpts = append(cOpts,
			containerd.WithImage(image),
			containerd.WithImageConfigLabels(image),
			containerd.WithSnapshotter(snapshotter),
			containerd.WithNewSnapshot(id, image),
			containerd.WithAdditionalContainerLabels(labels))

		if len(args) > 0 {
			opts = append(opts, oci.WithProcessArgs(args...))
		}
		if cwd := context.String("cwd"); cwd != "" {
			opts = append(opts, oci.WithProcessCwd(cwd))
		}
		if context.Bool("tty") {
			opts = append(opts, oci.WithTTY)

			con := console.Current()
			size, err := con.Size()
			if err != nil {
				logrus.WithError(err).Error("console size")
			}
			opts = append(opts, oci.WithTTYSize(int(size.Width), int(size.Height)))
		}
		if context.Bool("net-host") {
			return nil, errors.New("Cannot use host mode networking with Windows containers")
		}
		if context.Bool("isolated") {
			opts = append(opts, oci.WithWindowsHyperV)
		}
		limit := context.Uint64("memory-limit")
		if limit != 0 {
			opts = append(opts, oci.WithMemoryLimit(limit))
		}
		ccount := context.Uint64("cpu-count")
		if ccount != 0 {
			opts = append(opts, oci.WithWindowsCPUCount(ccount))
		}
	}

	runtime := context.String("runtime")
	var runtimeOpts interface{}
	if runtime == "io.containerd.runhcs.v1" {
		runtimeOpts = &options.Options{
			Debug: context.GlobalBool("debug"),
		}
	}
	cOpts = append(cOpts, containerd.WithRuntime(runtime, runtimeOpts))

	var s specs.Spec
	spec = containerd.WithSpec(&s, opts...)

	cOpts = append(cOpts, spec)

	return client.NewContainer(ctx, id, cOpts...)
}

func getNewTaskOpts(_ *cli.Context) []containerd.NewTaskOpts {
	return nil
}
