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

package opts

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/continuity/fs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// WithNewSnapshot wraps `containerd.WithNewSnapshot` so that if creating the
// snapshot fails we make sure the image is actually unpacked and and retry.
func WithNewSnapshot(id string, i containerd.Image) containerd.NewContainerOpts {
	f := containerd.WithNewSnapshot(id, i)
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) error {
		if err := f(ctx, client, c); err != nil {
			if !errdefs.IsNotFound(err) {
				return err
			}

			if err := i.Unpack(ctx, c.Snapshotter); err != nil {
				return errors.Wrap(err, "error unpacking image")
			}
			return f(ctx, client, c)
		}
		return nil
	}
}

// WithVolumes copies ownership of volume in rootfs to its corresponding host path.
// It doesn't update runtime spec.
// The passed in map is a host path to container path map for all volumes.
func WithVolumes(volumeMounts map[string]string) containerd.NewContainerOpts {
	return func(ctx context.Context, client *containerd.Client, c *containers.Container) (err error) {
		if c.Snapshotter == "" {
			return errors.New("no snapshotter set for container")
		}
		if c.SnapshotKey == "" {
			return errors.New("rootfs not created for container")
		}
		snapshotter := client.SnapshotService(c.Snapshotter)
		mounts, err := snapshotter.Mounts(ctx, c.SnapshotKey)
		if err != nil {
			return err
		}
		root, err := ioutil.TempDir("", "ctd-volume")
		if err != nil {
			return err
		}
		// We change RemoveAll to Remove so that we either leak a temp dir
		// if it fails but not RM snapshot data.
		// refer to https://github.com/containerd/containerd/pull/1868
		// https://github.com/containerd/containerd/pull/1785
		defer os.Remove(root) // nolint: errcheck
		if err := mount.All(mounts, root); err != nil {
			return errors.Wrap(err, "failed to mount")
		}
		defer func() {
			if uerr := mount.Unmount(root, 0); uerr != nil {
				logrus.WithError(uerr).Errorf("Failed to unmount snapshot %q", c.SnapshotKey)
				if err == nil {
					err = uerr
				}
			}
		}()

		for host, volume := range volumeMounts {
			src := filepath.Join(root, volume)
			if _, err := os.Stat(src); err != nil {
				if os.IsNotExist(err) {
					// Skip copying directory if it does not exist.
					continue
				}
				return errors.Wrap(err, "stat volume in rootfs")
			}
			if err := copyExistingContents(src, host); err != nil {
				return errors.Wrap(err, "taking runtime copy of volume")
			}
		}
		return nil
	}
}

// copyExistingContents copies from the source to the destination and
// ensures the ownership is appropriately set.
func copyExistingContents(source, destination string) error {
	dstList, err := ioutil.ReadDir(destination)
	if err != nil {
		return err
	}
	if len(dstList) != 0 {
		return errors.Errorf("volume at %q is not initially empty", destination)
	}
	return fs.CopyDir(destination, source)
}
