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

package os

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/mount"
	"github.com/containerd/fifo"
	"github.com/docker/docker/pkg/symlink"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
)

// OS collects system level operations that need to be mocked out
// during tests.
type OS interface {
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
	OpenFifo(ctx context.Context, fn string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
	Stat(name string) (os.FileInfo, error)
	ResolveSymbolicLink(name string) (string, error)
	FollowSymlinkInScope(path, scope string) (string, error)
	CopyFile(src, dest string, perm os.FileMode) error
	WriteFile(filename string, data []byte, perm os.FileMode) error
	Mount(source string, target string, fstype string, flags uintptr, data string) error
	Unmount(target string) error
	LookupMount(path string) (mount.Info, error)
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

// OpenFifo will call fifo.OpenFifo to open a fifo.
func (RealOS) OpenFifo(ctx context.Context, fn string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return fifo.OpenFifo(ctx, fn, flag, perm)
}

// Stat will call os.Stat to get the status of the given file.
func (RealOS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
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

// Mount will call unix.Mount to mount the file.
func (RealOS) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return unix.Mount(source, target, fstype, flags, data)
}

// Unmount will call Unmount to unmount the file.
func (RealOS) Unmount(target string) error {
	return Unmount(target)
}

// LookupMount gets mount info of a given path.
func (RealOS) LookupMount(path string) (mount.Info, error) {
	return mount.Lookup(path)
}

// Unmount unmounts the target. It does not return an error in case the target is not mounted.
// In case the target does not exist, the appropriate error is returned.
func Unmount(target string) error {
	err := unix.Unmount(target, unix.MNT_DETACH)
	if err == unix.EINVAL {
		// ignore "not mounted" error
		err = nil
	}

	return err
}

// Hostname will call os.Hostname to get the hostname of the host.
func (RealOS) Hostname() (string, error) {
	return os.Hostname()
}
