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
	"github.com/containerd/containerd/mount"
	"golang.org/x/sys/unix"
)

// Mount will call unix.Mount to mount the file.
func (RealOS) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return unix.Mount(source, target, fstype, flags, data)
}

// Unmount will call Unmount to unmount the file.
func (RealOS) Unmount(target string) error {
	return mount.Unmount(target, unix.MNT_DETACH)
}

// LookupMount gets mount info of a given path.
func (RealOS) LookupMount(path string) (mount.Info, error) {
	return mount.Lookup(path)
}
