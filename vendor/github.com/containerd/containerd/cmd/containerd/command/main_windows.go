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
	"fmt"
	"os"
	"strings"
	"unsafe"

	"github.com/Microsoft/go-winio/pkg/etw"
	"github.com/Microsoft/go-winio/pkg/etwlogrus"
	"github.com/Microsoft/go-winio/pkg/guid"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/services/server"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var (
	handledSignals = []os.Signal{
		windows.SIGTERM,
		windows.SIGINT,
	}
)

func handleSignals(ctx context.Context, signals chan os.Signal, serverC chan *server.Server) chan struct{} {
	done := make(chan struct{})
	go func() {
		var server *server.Server
		for {
			select {
			case s := <-serverC:
				server = s
			case s := <-signals:
				log.G(ctx).WithField("signal", s).Debug("received signal")

				if err := notifyStopping(ctx); err != nil {
					log.G(ctx).WithError(err).Error("notify stopping failed")
				}

				if server == nil {
					close(done)
					return
				}
				server.Stop()
				close(done)
			}
		}
	}()
	setupDumpStacks()
	return done
}

func setupDumpStacks() {
	// Windows does not support signals like *nix systems. So instead of
	// trapping on SIGUSR1 to dump stacks, we wait on a Win32 event to be
	// signaled. ACL'd to builtin administrators and local system
	event := "Global\\stackdump-" + fmt.Sprint(os.Getpid())
	ev, _ := windows.UTF16PtrFromString(event)
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;BA)(A;;GA;;;SY)")
	if err != nil {
		logrus.Errorf("failed to get security descriptor for debug stackdump event %s: %s", event, err.Error())
		return
	}
	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	sa.SecurityDescriptor = sd
	h, err := windows.CreateEvent(&sa, 0, 0, ev)
	if h == 0 || err != nil {
		logrus.Errorf("failed to create debug stackdump event %s: %s", event, err.Error())
		return
	}
	go func() {
		logrus.Debugf("Stackdump - waiting signal at %s", event)
		for {
			windows.WaitForSingleObject(h, windows.INFINITE)
			dumpStacks(true)
		}
	}()
}

func etwCallback(sourceID guid.GUID, state etw.ProviderState, level etw.Level, matchAnyKeyword uint64, matchAllKeyword uint64, filterData uintptr) {
	if state == etw.ProviderStateCaptureState {
		dumpStacks(false)
	}
}

func init() {
	// Provider ID: 2acb92c0-eb9b-571a-69cf-8f3410f383ad
	// Provider and hook aren't closed explicitly, as they will exist until
	// process exit. GUID is generated based on name - see
	// Microsoft/go-winio/tools/etw-provider-gen.
	provider, err := etw.NewProvider("ContainerD", etwCallback)
	if err != nil {
		logrus.Error(err)
	} else {
		if hook, err := etwlogrus.NewHookFromProvider(provider); err == nil {
			logrus.AddHook(hook)
		} else {
			logrus.Error(err)
		}
	}
}

func isLocalAddress(path string) bool {
	return strings.HasPrefix(path, `\\.\pipe\`)
}
