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

package images

import (
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/platforms"
	"github.com/opencontainers/image-spec/identity"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var mountCommand = cli.Command{
	Name:      "mount",
	Usage:     "mount an image to a target path",
	ArgsUsage: "[flags] <ref> <target>",
	Description: `Mount an image rootfs to a specified path.

When you are done, use the unmount command.
`,
	Flags: append(append(commands.RegistryFlags, append(commands.SnapshotterFlags, commands.LabelFlag)...),
		cli.BoolFlag{
			Name:  "rw",
			Usage: "Enable write support on the mount",
		},
		cli.StringFlag{
			Name:  "platform",
			Usage: "Mount the image for the specified platform",
			Value: platforms.DefaultString(),
		},
	),
	Action: func(context *cli.Context) (retErr error) {
		var (
			ref    = context.Args().First()
			target = context.Args().Get(1)
		)
		if ref == "" {
			return fmt.Errorf("please provide an image reference to mount")
		}
		if target == "" {
			return fmt.Errorf("please provide a target path to mount to")
		}

		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()

		snapshotter := context.GlobalString("snapshotter")
		if snapshotter == "" {
			snapshotter = containerd.DefaultSnapshotter
		}

		ctx, done, err := client.WithLease(ctx,
			leases.WithID(target),
			leases.WithExpiration(24*time.Hour),
			leases.WithLabels(map[string]string{
				"containerd.io/gc.ref.snapshot." + snapshotter: target,
			}),
		)
		if err != nil && !errdefs.IsAlreadyExists(err) {
			return err
		}

		defer func() {
			if retErr != nil && done != nil {
				done(ctx)
			}
		}()

		ps := context.String("platform")
		p, err := platforms.Parse(ps)
		if err != nil {
			return errors.Wrapf(err, "unable to parse platform %s", ps)
		}

		img, err := client.ImageService().Get(ctx, ref)
		if err != nil {
			return err
		}

		i := containerd.NewImageWithPlatform(client, img, platforms.Only(p))
		if err := i.Unpack(ctx, snapshotter); err != nil {
			return errors.Wrap(err, "error unpacking image")
		}

		diffIDs, err := i.RootFS(ctx)
		if err != nil {
			return err
		}
		chainID := identity.ChainID(diffIDs).String()
		fmt.Println(chainID)

		s := client.SnapshotService(snapshotter)

		var mounts []mount.Mount
		if context.Bool("rw") {
			mounts, err = s.Prepare(ctx, target, chainID)
		} else {
			mounts, err = s.View(ctx, target, chainID)
		}
		if err != nil {
			if errdefs.IsAlreadyExists(err) {
				mounts, err = s.Mounts(ctx, target)
			}
			if err != nil {
				return err
			}
		}

		if err := mount.All(mounts, target); err != nil {
			if err := s.Remove(ctx, target); err != nil && !errdefs.IsNotFound(err) {
				fmt.Fprintln(context.App.ErrWriter, "Error cleaning up snapshot after mount error:", err)
			}
			return err
		}

		fmt.Fprintln(context.App.Writer, target)
		return nil
	},
}
