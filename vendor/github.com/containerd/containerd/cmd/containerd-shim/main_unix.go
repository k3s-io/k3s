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

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/process"
	shimlog "github.com/containerd/containerd/runtime/v1"
	"github.com/containerd/containerd/runtime/v1/shim"
	shimapi "github.com/containerd/containerd/runtime/v1/shim/v1"
	"github.com/containerd/containerd/sys/reaper"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	debugFlag            bool
	namespaceFlag        string
	socketFlag           string
	addressFlag          string
	workdirFlag          string
	runtimeRootFlag      string
	criuFlag             string
	systemdCgroupFlag    bool
	containerdBinaryFlag string

	bufPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(nil)
		},
	}
)

func init() {
	flag.BoolVar(&debugFlag, "debug", false, "enable debug output in logs")
	flag.StringVar(&namespaceFlag, "namespace", "", "namespace that owns the shim")
	flag.StringVar(&socketFlag, "socket", "", "socket path to serve")
	flag.StringVar(&addressFlag, "address", "", "grpc address back to main containerd")
	flag.StringVar(&workdirFlag, "workdir", "", "path used to storge large temporary data")
	flag.StringVar(&runtimeRootFlag, "runtime-root", process.RuncRoot, "root directory for the runtime")
	flag.StringVar(&criuFlag, "criu", "", "path to criu binary")
	flag.BoolVar(&systemdCgroupFlag, "systemd-cgroup", false, "set runtime to use systemd-cgroup")
	// currently, the `containerd publish` utility is embedded in the daemon binary.
	// The daemon invokes `containerd-shim -containerd-binary ...` with its own os.Executable() path.
	flag.StringVar(&containerdBinaryFlag, "containerd-binary", "containerd", "path to containerd binary (used for `containerd publish`)")
	flag.Parse()
}

func main() {
	debug.SetGCPercent(40)
	go func() {
		for range time.Tick(30 * time.Second) {
			debug.FreeOSMemory()
		}
	}()

	if debugFlag {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if os.Getenv("GOMAXPROCS") == "" {
		// If GOMAXPROCS hasn't been set, we default to a value of 2 to reduce
		// the number of Go stacks present in the shim.
		runtime.GOMAXPROCS(2)
	}

	stdout, stderr, err := openStdioKeepAlivePipes(workdirFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "containerd-shim: %s\n", err)
		os.Exit(1)
	}
	defer func() {
		stdout.Close()
		stderr.Close()
	}()

	// redirect the following output into fifo to make sure that containerd
	// still can read the log after restart
	logrus.SetOutput(stdout)

	if err := executeShim(); err != nil {
		fmt.Fprintf(os.Stderr, "containerd-shim: %s\n", err)
		os.Exit(1)
	}
}

// If containerd server process dies, we need the shim to keep stdout/err reader
// FDs so that Linux does not SIGPIPE the shim process if it tries to use its end of
// these pipes.
func openStdioKeepAlivePipes(dir string) (io.ReadWriteCloser, io.ReadWriteCloser, error) {
	background := context.Background()
	keepStdoutAlive, err := shimlog.OpenShimStdoutLog(background, dir)
	if err != nil {
		return nil, nil, err
	}
	keepStderrAlive, err := shimlog.OpenShimStderrLog(background, dir)
	if err != nil {
		return nil, nil, err
	}
	return keepStdoutAlive, keepStderrAlive, nil
}

func executeShim() error {
	// start handling signals as soon as possible so that things are properly reaped
	// or if runtime exits before we hit the handler
	signals, err := setupSignals()
	if err != nil {
		return err
	}
	dump := make(chan os.Signal, 32)
	signal.Notify(dump, syscall.SIGUSR1)

	path, err := os.Getwd()
	if err != nil {
		return err
	}
	server, err := newServer()
	if err != nil {
		return errors.Wrap(err, "failed creating server")
	}
	sv, err := shim.NewService(
		shim.Config{
			Path:          path,
			Namespace:     namespaceFlag,
			WorkDir:       workdirFlag,
			Criu:          criuFlag,
			SystemdCgroup: systemdCgroupFlag,
			RuntimeRoot:   runtimeRootFlag,
		},
		&remoteEventsPublisher{address: addressFlag},
	)
	if err != nil {
		return err
	}
	logrus.Debug("registering ttrpc server")
	shimapi.RegisterShimService(server, sv)

	socket := socketFlag
	if err := serve(context.Background(), server, socket); err != nil {
		return err
	}
	logger := logrus.WithFields(logrus.Fields{
		"pid":       os.Getpid(),
		"path":      path,
		"namespace": namespaceFlag,
	})
	go func() {
		for range dump {
			dumpStacks(logger)
		}
	}()
	return handleSignals(logger, signals, server, sv)
}

// serve serves the ttrpc API over a unix socket at the provided path
// this function does not block
func serve(ctx context.Context, server *ttrpc.Server, path string) error {
	var (
		l   net.Listener
		err error
	)
	if path == "" {
		f := os.NewFile(3, "socket")
		l, err = net.FileListener(f)
		f.Close()
		path = "[inherited from parent]"
	} else {
		const (
			abstractSocketPrefix = "\x00"
			socketPathLimit      = 106
		)
		p := strings.TrimPrefix(path, "unix://")
		if len(p) == len(path) {
			p = abstractSocketPrefix + p
		}
		if len(p) > socketPathLimit {
			return errors.Errorf("%q: unix socket path too long (> %d)", p, socketPathLimit)
		}
		l, err = net.Listen("unix", p)
	}
	if err != nil {
		return err
	}
	logrus.WithField("socket", path).Debug("serving api on unix socket")
	go func() {
		defer l.Close()
		if err := server.Serve(ctx, l); err != nil &&
			!strings.Contains(err.Error(), "use of closed network connection") {
			logrus.WithError(err).Fatal("containerd-shim: ttrpc server failure")
		}
	}()
	return nil
}

func handleSignals(logger *logrus.Entry, signals chan os.Signal, server *ttrpc.Server, sv *shim.Service) error {
	var (
		termOnce sync.Once
		done     = make(chan struct{})
	)

	for {
		select {
		case <-done:
			return nil
		case s := <-signals:
			switch s {
			case unix.SIGCHLD:
				if err := reaper.Reap(); err != nil {
					logger.WithError(err).Error("reap exit status")
				}
			case unix.SIGTERM, unix.SIGINT:
				go termOnce.Do(func() {
					ctx := context.TODO()
					if err := server.Shutdown(ctx); err != nil {
						logger.WithError(err).Error("failed to shutdown server")
					}
					// Ensure our child is dead if any
					sv.Kill(ctx, &shimapi.KillRequest{
						Signal: uint32(syscall.SIGKILL),
						All:    true,
					})
					sv.Delete(context.Background(), &ptypes.Empty{})
					close(done)
				})
			case unix.SIGPIPE:
			}
		}
	}
}

func dumpStacks(logger *logrus.Entry) {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	logger.Infof("=== BEGIN goroutine stack dump ===\n%s\n=== END goroutine stack dump ===", buf)
}

type remoteEventsPublisher struct {
	address string
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
	cmd := exec.CommandContext(ctx, containerdBinaryFlag, "--address", l.address, "publish", "--topic", topic, "--namespace", ns)
	cmd.Stdin = bytes.NewReader(data)
	b := bufPool.Get().(*bytes.Buffer)
	defer func() {
		b.Reset()
		bufPool.Put(b)
	}()
	cmd.Stdout = b
	cmd.Stderr = b
	c, err := reaper.Default.Start(cmd)
	if err != nil {
		return err
	}
	status, err := reaper.Default.Wait(cmd, c)
	if err != nil {
		return errors.Wrapf(err, "failed to publish event: %s", b.String())
	}
	if status != 0 {
		return errors.Errorf("failed to publish event: %s", b.String())
	}
	return nil
}
