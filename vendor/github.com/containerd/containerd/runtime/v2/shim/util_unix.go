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

package shim

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/sys"
	"github.com/pkg/errors"
)

func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// SetScore sets the oom score for a process
func SetScore(pid int) error {
	return sys.SetOOMScore(pid, sys.OOMScoreMaxKillable)
}

// SocketAddress returns an abstract socket address
func SocketAddress(ctx context.Context, id string) (string, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(string(filepath.Separator), "containerd-shim", ns, id, "shim.sock"), nil
}

// AnonDialer returns a dialer for an abstract socket
func AnonDialer(address string, timeout time.Duration) (net.Conn, error) {
	address = strings.TrimPrefix(address, "unix://")
	return net.DialTimeout("unix", "\x00"+address, timeout)
}

// NewSocket returns a new socket
func NewSocket(address string) (*net.UnixListener, error) {
	if len(address) > 106 {
		return nil, errors.Errorf("%q: unix socket path too long (> 106)", address)
	}
	l, err := net.Listen("unix", "\x00"+address)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to listen to abstract unix socket %q", address)
	}
	return l.(*net.UnixListener), nil
}
