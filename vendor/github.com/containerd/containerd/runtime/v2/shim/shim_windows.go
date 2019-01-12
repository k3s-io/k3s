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

package shim

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	winio "github.com/Microsoft/go-winio"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/ttrpc"
	"github.com/containerd/typeurl"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const (
	errorConnectionAborted syscall.Errno = 1236
)

// setupSignals creates a new signal handler for all signals
func setupSignals() (chan os.Signal, error) {
	signals := make(chan os.Signal, 32)
	return signals, nil
}

func newServer() (*ttrpc.Server, error) {
	return ttrpc.NewServer()
}

func subreaper() error {
	return nil
}

type fakeSignal struct {
}

func (fs *fakeSignal) String() string {
	return ""
}

func (fs *fakeSignal) Signal() {
}

func setupDumpStacks(dump chan<- os.Signal) {
	// Windows does not support signals like *nix systems. So instead of
	// trapping on SIGUSR1 to dump stacks, we wait on a Win32 event to be
	// signaled. ACL'd to builtin administrators and local system
	event := "Global\\containerd-shim-runhcs-v1-" + fmt.Sprint(os.Getpid())
	ev, _ := windows.UTF16PtrFromString(event)
	sd, err := winio.SddlToSecurityDescriptor("D:P(A;;GA;;;BA)(A;;GA;;;SY)")
	if err != nil {
		logrus.Errorf("failed to get security descriptor for debug stackdump event %s: %s", event, err.Error())
		return
	}
	var sa windows.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	sa.SecurityDescriptor = uintptr(unsafe.Pointer(&sd[0]))
	h, err := windows.CreateEvent(&sa, 0, 0, ev)
	if h == 0 || err != nil {
		logrus.Errorf("failed to create debug stackdump event %s: %s", event, err.Error())
		return
	}
	go func() {
		logrus.Debugf("Stackdump - waiting signal at %s", event)
		for {
			windows.WaitForSingleObject(h, windows.INFINITE)
			dump <- new(fakeSignal)
		}
	}()
}

// serve serves the ttrpc API over a unix socket at the provided path
// this function does not block
func serveListener(path string) (net.Listener, error) {
	if path == "" {
		return nil, errors.New("'socket' must be npipe path")
	}
	l, err := winio.ListenPipe(path, nil)
	if err != nil {
		return nil, err
	}
	logrus.WithField("socket", path).Debug("serving api on npipe socket")
	return l, nil
}

func handleSignals(logger *logrus.Entry, signals chan os.Signal) error {
	logger.Info("starting signal loop")
	for {
		select {
		case s := <-signals:
			switch s {
			case os.Interrupt:
				break
			}
		}
	}
}

var _ = (io.WriterTo)(&blockingBuffer{})
var _ = (io.Writer)(&blockingBuffer{})

// blockingBuffer implements the `io.Writer` and `io.WriterTo` interfaces. Once
// `capacity` is reached the calls to `Write` will block until a successful call
// to `WriterTo` frees up the buffer space.
//
// Note: This has the same threadding semantics as bytes.Buffer with no
// additional locking so multithreading is not supported.
type blockingBuffer struct {
	c *sync.Cond

	capacity int

	buffer bytes.Buffer
}

func newBlockingBuffer(capacity int) *blockingBuffer {
	return &blockingBuffer{
		c:        sync.NewCond(&sync.Mutex{}),
		capacity: capacity,
	}
}

func (bb *blockingBuffer) Len() int {
	bb.c.L.Lock()
	defer bb.c.L.Unlock()
	return bb.buffer.Len()
}

func (bb *blockingBuffer) Write(p []byte) (int, error) {
	if len(p) > bb.capacity {
		return 0, errors.Errorf("len(p) (%d) too large for capacity (%d)", len(p), bb.capacity)
	}

	bb.c.L.Lock()
	for bb.buffer.Len()+len(p) > bb.capacity {
		bb.c.Wait()
	}
	defer bb.c.L.Unlock()
	return bb.buffer.Write(p)
}

func (bb *blockingBuffer) WriteTo(w io.Writer) (int64, error) {
	bb.c.L.Lock()
	defer bb.c.L.Unlock()
	defer bb.c.Signal()
	return bb.buffer.WriteTo(w)
}

// deferredShimWriteLogger exists to solve the upstream loggin issue presented
// by using Windows Named Pipes for logging. When containerd restarts it tries
// to reconnect to any shims. This means that the connection to the logger will
// be severed but when containerd starts up it should reconnect and start
// logging again. We abstract all of this logic behind what looks like a simple
// `io.Writer` that can reconnect in the lifetime and buffers logs while
// disconnected.
type deferredShimWriteLogger struct {
	mu sync.Mutex

	ctx context.Context

	connected bool
	aborted   bool

	buffer *blockingBuffer

	l      net.Listener
	c      net.Conn
	conerr error
}

// beginAccept issues an accept to wait for a connection. Once a connection
// occurs drains any outstanding buffer. While draining the buffer any writes
// are blocked. If the buffer fails to fully drain due to a connection drop a
// call to `beginAccept` is re-issued waiting for another connection from
// containerd.
func (dswl *deferredShimWriteLogger) beginAccept() {
	dswl.mu.Lock()
	if dswl.connected {
		return
	}
	dswl.mu.Unlock()

	c, err := dswl.l.Accept()
	if err == errorConnectionAborted {
		dswl.mu.Lock()
		dswl.aborted = true
		dswl.l.Close()
		dswl.conerr = errors.New("connection closed")
		dswl.mu.Unlock()
		return
	}
	dswl.mu.Lock()
	dswl.connected = true
	dswl.c = c

	// Drain the buffer
	if dswl.buffer.Len() > 0 {
		_, err := dswl.buffer.WriteTo(dswl.c)
		if err != nil {
			// We lost our connection draining the buffer.
			dswl.connected = false
			dswl.c.Close()
			go dswl.beginAccept()
		}
	}
	dswl.mu.Unlock()
}

func (dswl *deferredShimWriteLogger) Write(p []byte) (int, error) {
	dswl.mu.Lock()
	defer dswl.mu.Unlock()

	if dswl.aborted {
		return 0, dswl.conerr
	}

	if dswl.connected {
		// We have a connection. beginAccept would have drained the buffer so we just write our data to
		// the connection directly.
		written, err := dswl.c.Write(p)
		if err != nil {
			// We lost the connection.
			dswl.connected = false
			dswl.c.Close()
			go dswl.beginAccept()

			// We weren't able to write the full `p` bytes. Buffer the rest
			if written != len(p) {
				w, err := dswl.buffer.Write(p[written:])
				if err != nil {
					// We failed to buffer. Return this error
					return written + w, err
				}
				written += w
			}
		}

		return written, nil
	}

	// We are disconnected. Buffer the contents.
	return dswl.buffer.Write(p)
}

// openLog on Windows acts as the server of the log pipe. This allows the
// containerd daemon to independently restart and reconnect to the logs.
func openLog(ctx context.Context, id string) (io.Writer, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}

	dswl := &deferredShimWriteLogger{
		ctx:    ctx,
		buffer: newBlockingBuffer(64 * 1024), // 64KB,
	}
	l, err := winio.ListenPipe(fmt.Sprintf("\\\\.\\pipe\\containerd-shim-%s-%s-log", ns, id), nil)
	if err != nil {
		return nil, err
	}
	dswl.l = l
	go dswl.beginAccept()
	return dswl, nil
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
	return cmd.Run()
}
