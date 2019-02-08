// +build !windows

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
	"path/filepath"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/contrib/nvidia"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// NewContainer creates a new container
func NewContainer(ctx gocontext.Context, client *containerd.Client, context *cli.Context) (containerd.Container, error) {
	var (
		id     string
		config = context.IsSet("config")
	)
	if config {
		id = context.Args().First()
	} else {
		id = context.Args().Get(1)
	}

	if raw := context.String("checkpoint"); raw != "" {
		im, err := client.GetImage(ctx, raw)
		if err != nil {
			return nil, err
		}
		return client.NewContainer(ctx, id, containerd.WithCheckpoint(im, id), containerd.WithRuntime(context.String("runtime"), nil))
	}

	var (
		opts  []oci.SpecOpts
		cOpts []containerd.NewContainerOpts
		spec  containerd.NewContainerOpts
	)

	cOpts = append(cOpts, containerd.WithContainerLabels(commands.LabelArgs(context.StringSlice("label"))))
	if config {
		opts = append(opts, oci.WithSpecFromFile(context.String("config")))
	} else {
		var (
			ref = context.Args().First()
			//for container's id is Args[1]
			args = context.Args()[2:]
		)
		opts = append(opts, oci.WithDefaultSpec(), oci.WithDefaultUnixDevices)
		opts = append(opts, oci.WithEnv(context.StringSlice("env")))
		opts = append(opts, withMounts(context))

		if context.Bool("rootfs") {
			rootfs, err := filepath.Abs(ref)
			if err != nil {
				return nil, err
			}
			opts = append(opts, oci.WithRootFSPath(rootfs))
		} else {
			snapshotter := context.String("snapshotter")
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
			cOpts = append(cOpts,
				containerd.WithImage(image),
				containerd.WithSnapshotter(snapshotter),
				// Even when "readonly" is set, we don't use KindView snapshot here. (#1495)
				// We pass writable snapshot to the OCI runtime, and the runtime remounts it as read-only,
				// after creating some mount points on demand.
				containerd.WithNewSnapshot(id, image),
				containerd.WithImageStopSignal(image, "SIGTERM"))
		}
		if context.Bool("readonly") {
			opts = append(opts, oci.WithRootFSReadonly())
		}
		if len(args) > 0 {
			opts = append(opts, oci.WithProcessArgs(args...))
		}
		if cwd := context.String("cwd"); cwd != "" {
			opts = append(opts, oci.WithProcessCwd(cwd))
		}
		if context.Bool("tty") {
			opts = append(opts, oci.WithTTY)
		}
		if context.Bool("privileged") {
			opts = append(opts, oci.WithPrivileged)
		}
		if context.Bool("net-host") {
			opts = append(opts, oci.WithHostNamespace(specs.NetworkNamespace), oci.WithHostHostsFile, oci.WithHostResolvconf)
		}

		joinNs := context.StringSlice("with-ns")
		for _, ns := range joinNs {
			parts := strings.Split(ns, ":")
			if len(parts) != 2 {
				return nil, errors.New("joining a Linux namespace using --with-ns requires the format 'nstype:path'")
			}
			if !validNamespace(parts[0]) {
				return nil, errors.New("the Linux namespace type specified in --with-ns is not valid: " + parts[0])
			}
			opts = append(opts, oci.WithLinuxNamespace(specs.LinuxNamespace{
				Type: specs.LinuxNamespaceType(parts[0]),
				Path: parts[1],
			}))
		}
		if context.IsSet("gpus") {
			opts = append(opts, nvidia.WithGPUs(nvidia.WithDevices(context.Int("gpus")), nvidia.WithAllCapabilities))
		}
		if context.IsSet("allow-new-privs") {
			opts = append(opts, oci.WithNewPrivileges)
		}
	}

	cOpts = append(cOpts, containerd.WithRuntime(context.String("runtime"), nil))

	var s specs.Spec
	spec = containerd.WithSpec(&s, opts...)

	cOpts = append(cOpts, spec)

	// oci.WithImageConfig (WithUsername, WithUserID) depends on access to rootfs for resolving via
	// the /etc/{passwd,group} files. So cOpts needs to have precedence over opts.
	return client.NewContainer(ctx, id, cOpts...)
}

func getNewTaskOpts(context *cli.Context) []containerd.NewTaskOpts {
	if context.Bool("no-pivot") {
		return []containerd.NewTaskOpts{containerd.WithNoPivotRoot}
	}
	return nil
}

func validNamespace(ns string) bool {
	linuxNs := specs.LinuxNamespaceType(ns)
	switch linuxNs {
	case specs.PIDNamespace,
		specs.NetworkNamespace,
		specs.UTSNamespace,
		specs.MountNamespace,
		specs.UserNamespace,
		specs.IPCNamespace,
		specs.CgroupNamespace:
		return true
	default:
		return false
	}
}
