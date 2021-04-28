// +build linux

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

package fuseoverlayfs

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/pkg/errors"
)

// supportsReadonlyMultipleLowerDir checks if read-only multiple lowerdirs can be mounted with fuse-overlayfs.
// https://github.com/containers/fuse-overlayfs/pull/133
func supportsReadonlyMultipleLowerDir(d string) error {
	td, err := ioutil.TempDir(d, "fuseoverlayfs-check")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(td); err != nil {
			log.L.WithError(err).Warnf("Failed to remove check directory %v", td)
		}
	}()

	for _, dir := range []string{"lower1", "lower2", "merged"} {
		if err := os.Mkdir(filepath.Join(td, dir), 0755); err != nil {
			return err
		}
	}

	opts := []string{fmt.Sprintf("lowerdir=%s:%s", filepath.Join(td, "lower2"), filepath.Join(td, "lower1"))}
	m := mount.Mount{
		Type:    "fuse3." + fuseoverlayfsBinary,
		Source:  "overlay",
		Options: opts,
	}
	dest := filepath.Join(td, "merged")
	if err := m.Mount(dest); err != nil {
		return errors.Wrapf(err, "failed to mount fuse-overlayfs (%+v) on %s", m, dest)
	}
	if err := mount.UnmountAll(dest, 0); err != nil {
		log.L.WithError(err).Warnf("Failed to unmount check directory %v", dest)
	}
	return nil
}

// Supported returns nil when the overlayfs is functional on the system with the root directory.
// Supported is not called during plugin initialization, but exposed for downstream projects which uses
// this snapshotter as a library.
func Supported(root string) error {
	if _, err := exec.LookPath(fuseoverlayfsBinary); err != nil {
		return errors.Wrapf(err, "%s not installed", fuseoverlayfsBinary)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if err := supportsReadonlyMultipleLowerDir(root); err != nil {
		return errors.Wrap(err, "fuse-overlayfs not functional, make sure running with kernel >= 4.18")
	}
	return nil
}
