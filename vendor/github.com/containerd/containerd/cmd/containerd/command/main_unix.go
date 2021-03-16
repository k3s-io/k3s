// +build linux darwin freebsd solaris

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

package command

import (
	"context"
	"os"

	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/services/server"
	"golang.org/x/sys/unix"
)

const defaultConfigPath = "/etc/containerd/config.toml"

var handledSignals = []os.Signal{
	unix.SIGTERM,
	unix.SIGINT,
	unix.SIGUSR1,
	unix.SIGPIPE,
}

func handleSignals(ctx context.Context, signals chan os.Signal, serverC chan *server.Server) chan struct{} {
	done := make(chan struct{}, 1)
	go func() {
		var server *server.Server
		for {
			select {
			case s := <-serverC:
				server = s
			case s := <-signals:
				log.G(ctx).WithField("signal", s).Debug("received signal")
				switch s {
				case unix.SIGUSR1:
					dumpStacks(true)
				case unix.SIGPIPE:
					continue
				default:
					if err := notifyStopping(ctx); err != nil {
						log.G(ctx).WithError(err).Error("notify stopping failed")
					}

					if server == nil {
						close(done)
						return
					}
					server.Stop()
					close(done)
					return
				}
			}
		}
	}()
	return done
}
