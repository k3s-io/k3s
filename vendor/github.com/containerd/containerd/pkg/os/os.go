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
	"io"
	"io/ioutil"
	"os"

	"github.com/moby/sys/symlink"
)

// OS collects system level operations that need to be mocked out
// during tests.
type OS interface {
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
	Stat(name string) (os.FileInfo, error)
	ResolveSymbolicLink(name string) (string, error)
	FollowSymlinkInScope(path, scope string) (string, error)
	CopyFile(src, dest string, perm os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
	Hostname() (string, error)
}

// RealOS is used to dispatch the real system level operations.
type RealOS struct{}

// MkdirAll will call os.MkdirAll to create a directory.
func (RealOS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// RemoveAll will call os.RemoveAll to remove the path and its children.
func (RealOS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Stat will call os.Stat to get the status of the given file.
func (RealOS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// FollowSymlinkInScope will call symlink.FollowSymlinkInScope.
func (RealOS) FollowSymlinkInScope(path, scope string) (string, error) {
	return symlink.FollowSymlinkInScope(path, scope)
}

// CopyFile will copy src file to dest file
func (RealOS) CopyFile(src, dest string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// WriteFile will call ioutil.WriteFile to write data into a file.
func (RealOS) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(filename, data, perm)
}

// Hostname will call os.Hostname to get the hostname of the host.
func (RealOS) Hostname() (string, error) {
	return os.Hostname()
}
