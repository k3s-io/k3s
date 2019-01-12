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
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/fifo"
	"github.com/containerd/typeurl"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// setupSignals creates a new signal handler for all signals and sets the shim as a
// sub-reaper so that the container processes are reparented
func setupSignals() (chan os.Signal, error) {
	signals := make(chan os.Signal, 32)
	signal.Notify(signals, unix.SIGTERM, unix.SIGINT, unix.SIGCHLD, unix.SIGPIPE)
	return signals, nil
}

func setupDumpStacks(dump chan<- os.Signal) {
	signal.Notify(dump, syscall.SIGUSR1)
}

func serveListener(path string) (net.Listener, error) {
	var (
		l   net.Listener
		err error
	)
	if path == "" {
		l, err = net.FileListener(os.NewFile(3, "socket"))
		path = "[inherited from parent]"
	} else {
		if len(path) > 106 {
			return nil, errors.Errorf("%q: unix socket path too long (> 106)", path)
		}
		l, err = net.Listen("unix", "\x00"+path)
	}
	if err != nil {
		return nil, err
	}
	logrus.WithField("socket", path).Debug("serving api on abstract socket")
	return l, nil
}

func handleSignals(logger *logrus.Entry, signals chan os.Signal) error {
	logger.Info("starting signal loop")
	for {
		select {
		case s := <-signals:
			switch s {
			case unix.SIGCHLD:
				if err := Reap(); err != nil {
					logger.WithError(err).Error("reap exit status")
				}
			case unix.SIGPIPE:
			}
		}
	}
}

func openLog(ctx context.Context, _ string) (io.Writer, error) {
	return fifo.OpenFifo(ctx, "log", unix.O_WRONLY, 0700)
}

func (l *remoteEventsPublisher) Publish(ctx context.Context, topic string, event events.Event) error {
	ns, _ := namespaces.Namespace(ctx)
	encoded, err := typeurl.MarshalAny(event)
	if err != nil {
		return err
	}
	data, err := encoded.Marshal()
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, l.containerdBinaryPath, "--address", l.address, "publish", "--topic", topic, "--namespace", ns)
	cmd.Stdin = bytes.NewReader(data)
	c, err := Default.Start(cmd)
	if err != nil {
		return err
	}
	status, err := Default.Wait(cmd, c)
	if err != nil {
		return err
	}
	if status != 0 {
		return errors.New("failed to publish event")
	}
	return nil
}
