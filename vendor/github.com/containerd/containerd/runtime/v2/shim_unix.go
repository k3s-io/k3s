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

package v2

import (
	"context"
	"io"
	"net"
	"path/filepath"
	"time"

	"github.com/containerd/fifo"
	"golang.org/x/sys/unix"
)

func openShimLog(ctx context.Context, bundle *Bundle, _ func(string, time.Duration) (net.Conn, error)) (io.ReadCloser, error) {
	return fifo.OpenFifo(ctx, filepath.Join(bundle.Path, "log"), unix.O_RDWR|unix.O_CREAT|unix.O_NONBLOCK, 0700)
}

func checkCopyShimLogError(ctx context.Context, err error) error {
	// When using a multi-container shim, the fifo of the 2nd to Nth
	// container will not be opened when the ctx is done. This will
	// cause an ErrReadClosed that can be ignored.
	select {
	case <-ctx.Done():
		if err == fifo.ErrReadClosed {
			return nil
		}
	default:
	}
	return err
}
