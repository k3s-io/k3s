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

// Copyright 2018 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software

// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package netns

import (
	"crypto/rand"
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"

	"github.com/containerd/containerd/mount"
	cnins "github.com/containernetworking/plugins/pkg/ns"
	"github.com/moby/sys/symlink"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Some of the following functions are migrated from
// https://github.com/containernetworking/plugins/blob/master/pkg/testutils/netns_linux.go

// newNS creates a new persistent (bind-mounted) network namespace and returns the
// path to the network namespace.
func newNS(baseDir string) (nsPath string, err error) {
	b := make([]byte, 16)
	if _, err := rand.Reader.Read(b); err != nil {
		return "", errors.Wrap(err, "failed to generate random netns name")
	}

	// Create the directory for mounting network namespaces
	// This needs to be a shared mountpoint in case it is mounted in to
	// other namespaces (containers)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", err
	}

	// create an empty file at the mount point
	nsName := fmt.Sprintf("cni-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	nsPath = path.Join(baseDir, nsName)
	mountPointFd, err := os.Create(nsPath)
	if err != nil {
		return "", err
	}
	mountPointFd.Close()

	defer func() {
		// Ensure the mount point is cleaned up on errors
		if err != nil {
			os.RemoveAll(nsPath) // nolint: errcheck
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	// do namespace work in a dedicated goroutine, so that we can safely
	// Lock/Unlock OSThread without upsetting the lock/unlock state of
	// the caller of this function
	go (func() {
		defer wg.Done()
		runtime.LockOSThread()
		// Don't unlock. By not unlocking, golang will kill the OS thread when the
		// goroutine is done (for go1.10+)

		var origNS cnins.NetNS
		origNS, err = cnins.GetNS(getCurrentThreadNetNSPath())
		if err != nil {
			return
		}
		defer origNS.Close()

		// create a new netns on the current thread
		err = unix.Unshare(unix.CLONE_NEWNET)
		if err != nil {
			return
		}

		// Put this thread back to the orig ns, since it might get reused (pre go1.10)
		defer origNS.Set() // nolint: errcheck

		// bind mount the netns from the current thread (from /proc) onto the
		// mount point. This causes the namespace to persist, even when there
		// are no threads in the ns.
		err = unix.Mount(getCurrentThreadNetNSPath(), nsPath, "none", unix.MS_BIND, "")
		if err != nil {
			err = errors.Wrapf(err, "failed to bind mount ns at %s", nsPath)
		}
	})()
	wg.Wait()

	if err != nil {
		return "", errors.Wrap(err, "failed to create namespace")
	}

	return nsPath, nil
}

// unmountNS unmounts the NS held by the netns object. unmountNS is idempotent.
func unmountNS(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "failed to stat netns")
	}
	path, err := symlink.FollowSymlinkInScope(path, "/")
	if err != nil {
		return errors.Wrap(err, "failed to follow symlink")
	}
	if err := mount.Unmount(path, unix.MNT_DETACH); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to umount netns")
	}
	if err := os.RemoveAll(path); err != nil {
		return errors.Wrap(err, "failed to remove netns")
	}
	return nil
}

// getCurrentThreadNetNSPath copied from pkg/ns
func getCurrentThreadNetNSPath() string {
	// /proc/self/ns/net returns the namespace of the main thread, not
	// of whatever thread this goroutine is running on.  Make sure we
	// use the thread's net namespace since the thread is switching around
	return fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), unix.Gettid())
}

// NetNS holds network namespace.
type NetNS struct {
	path string
}

// NewNetNS creates a network namespace.
func NewNetNS(baseDir string) (*NetNS, error) {
	path, err := newNS(baseDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup netns")
	}
	return &NetNS{path: path}, nil
}

// LoadNetNS loads existing network namespace.
func LoadNetNS(path string) *NetNS {
	return &NetNS{path: path}
}

// Remove removes network namespace. Remove is idempotent, meaning it might
// be invoked multiple times and provides consistent result.
func (n *NetNS) Remove() error {
	return unmountNS(n.path)
}

// Closed checks whether the network namespace has been closed.
func (n *NetNS) Closed() (bool, error) {
	ns, err := cnins.GetNS(n.path)
	if err != nil {
		if _, ok := err.(cnins.NSPathNotExistErr); ok {
			// The network namespace has already been removed.
			return true, nil
		}
		if _, ok := err.(cnins.NSPathNotNSErr); ok {
			// The network namespace is not mounted, remove it.
			if err := os.RemoveAll(n.path); err != nil {
				return false, errors.Wrap(err, "remove netns")
			}
			return true, nil
		}
		return false, errors.Wrap(err, "get netns fd")
	}
	if err := ns.Close(); err != nil {
		return false, errors.Wrap(err, "close netns fd")
	}
	return false, nil
}

// GetPath returns network namespace path for sandbox container
func (n *NetNS) GetPath() string {
	return n.path
}

// Do runs a function in the network namespace.
func (n *NetNS) Do(f func(cnins.NetNS) error) error {
	ns, err := cnins.GetNS(n.path)
	if err != nil {
		return errors.Wrap(err, "get netns fd")
	}
	defer ns.Close() // nolint: errcheck
	return ns.Do(f)
}
