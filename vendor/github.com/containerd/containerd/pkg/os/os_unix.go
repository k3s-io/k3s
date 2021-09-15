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

package os

import (
	"os"
	"path/filepath"

	"github.com/containerd/containerd/mount"
)

// UNIX collects unix system level operations that need to be
// mocked out during tests.
type UNIX interface {
	Mount(source string, target string, fstype string, flags uintptr, data string) error
	Unmount(target string) error
	LookupMount(path string) (mount.Info, error)
}

// ResolveSymbolicLink will follow any symbolic links
func (RealOS) ResolveSymbolicLink(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != os.ModeSymlink {
		return path, nil
	}
	return filepath.EvalSymlinks(path)
}
