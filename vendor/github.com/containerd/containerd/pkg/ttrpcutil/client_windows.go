// +build windows

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

package ttrpcutil

import (
	"context"
	"net"
	"os"
	"time"

	winio "github.com/Microsoft/go-winio"
	"github.com/pkg/errors"
)

func ttrpcDial(address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// If there is nobody serving the pipe we limit the timeout for this case to
	// 5 seconds because any shim that would serve this endpoint should serve it
	// within 5 seconds.
	serveTimer := time.NewTimer(5 * time.Second)
	defer serveTimer.Stop()
	for {
		c, err := winio.DialPipeContext(ctx, address)
		if err != nil {
			if os.IsNotExist(err) {
				select {
				case <-serveTimer.C:
					return nil, errors.Wrap(os.ErrNotExist, "pipe not found before timeout")
				default:
					// Wait 10ms for the shim to serve and try again.
					time.Sleep(10 * time.Millisecond)
					continue
				}
			} else if err == context.DeadlineExceeded {
				return nil, errors.Wrapf(err, "timed out waiting for npipe %s", address)
			}
			return nil, err
		}
		return c, nil
	}
}
