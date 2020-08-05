package parent

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/pkg/errors"

	"github.com/rootless-containers/rootlesskit/pkg/port"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/msg"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/opaque"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/parent/tcp"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/parent/udp"
	"github.com/rootless-containers/rootlesskit/pkg/port/portutil"
)

// NewDriver for builtin driver.
func NewDriver(logWriter io.Writer, stateDir string) (port.ParentDriver, error) {
	// TODO: consider using socketpair FD instead of socket file
	socketPath := filepath.Join(stateDir, ".bp.sock")
	childReadyPipePath := filepath.Join(stateDir, ".bp-ready.pipe")
	// remove the path just in case the previous rootlesskit instance crashed
	if err := os.RemoveAll(childReadyPipePath); err != nil {
		return nil, errors.Wrapf(err, "cannot remove %s", childReadyPipePath)
	}
	if err := syscall.Mkfifo(childReadyPipePath, 0600); err != nil {
		return nil, errors.Wrapf(err, "cannot mkfifo %s", childReadyPipePath)
	}
	d := driver{
		logWriter:          logWriter,
		socketPath:         socketPath,
		childReadyPipePath: childReadyPipePath,
		ports:              make(map[int]*port.Status, 0),
		stoppers:           make(map[int]func() error, 0),
		nextID:             1,
	}
	return &d, nil
}

type driver struct {
	logWriter          io.Writer
	socketPath         string
	childReadyPipePath string
	mu                 sync.Mutex
	ports              map[int]*port.Status
	stoppers           map[int]func() error
	nextID             int
}

func (d *driver) OpaqueForChild() map[string]string {
	return map[string]string{
		opaque.SocketPath:         d.socketPath,
		opaque.ChildReadyPipePath: d.childReadyPipePath,
	}
}

func (d *driver) RunParentDriver(initComplete chan struct{}, quit <-chan struct{}, _ *port.ChildContext) error {
	childReadyPipeR, err := os.OpenFile(d.childReadyPipePath, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		return err
	}
	if _, err = ioutil.ReadAll(childReadyPipeR); err != nil {
		return err
	}
	childReadyPipeR.Close()
	var dialer net.Dialer
	conn, err := dialer.Dial("unix", d.socketPath)
	if err != nil {
		return err
	}
	err = msg.Initiate(conn.(*net.UnixConn))
	conn.Close()
	if err != nil {
		return err
	}
	initComplete <- struct{}{}
	<-quit
	return nil
}

func isEPERM(err error) bool {
	k := "permission denied"
	// As of Go 1.14, errors.Is(err, syscall.EPERM) does not seem to work for
	// "listen tcp 0.0.0.0:80: bind: permission denied" error from net.ListenTCP().
	return errors.Is(err, syscall.EPERM) || strings.Contains(err.Error(), k)
}

// annotateEPERM annotates origErr for human-readability
func annotateEPERM(origErr error, spec port.Spec) error {
	// Read "net.ipv4.ip_unprivileged_port_start" value (typically 1024)
	// TODO: what for IPv6?
	// NOTE: sync.Once should not be used here
	b, e := ioutil.ReadFile("/proc/sys/net/ipv4/ip_unprivileged_port_start")
	if e != nil {
		return origErr
	}
	start, e := strconv.Atoi(strings.TrimSpace(string(b)))
	if e != nil {
		return origErr
	}
	if spec.ParentPort >= start {
		// origErr is unrelated to ip_unprivileged_port_start
		return origErr
	}
	text := fmt.Sprintf("cannot expose privileged port %d, you might need to add \"net.ipv4.ip_unprivileged_port_start=0\" (currently %d) to /etc/sysctl.conf", spec.ParentPort, start)
	if filepath.Base(os.Args[0]) == "rootlesskit" {
		// NOTE: The following sentence is appended only if Args[0] == "rootlesskit", because it does not apply to Podman (as of Podman v1.9).
		// Podman launches the parent driver in the child user namespace (but in the parent network namespace), which disables the file capability.
		text += ", or set CAP_NET_BIND_SERVICE on rootlesskit binary"
	}
	text += fmt.Sprintf(", or choose a larger port number (>= %d)", start)
	return errors.Wrap(origErr, text)
}

func (d *driver) AddPort(ctx context.Context, spec port.Spec) (*port.Status, error) {
	d.mu.Lock()
	err := portutil.ValidatePortSpec(spec, d.ports)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	routineStopCh := make(chan struct{})
	routineStop := func() error {
		close(routineStopCh)
		return nil // FIXME
	}
	switch spec.Proto {
	case "tcp":
		err = tcp.Run(d.socketPath, spec, routineStopCh, d.logWriter)
	case "udp":
		err = udp.Run(d.socketPath, spec, routineStopCh, d.logWriter)
	default:
		// NOTREACHED
		return nil, errors.New("spec was not validated?")
	}
	if err != nil {
		if isEPERM(err) {
			err = annotateEPERM(err, spec)
		}
		return nil, err
	}
	d.mu.Lock()
	id := d.nextID
	st := port.Status{
		ID:   id,
		Spec: spec,
	}
	d.ports[id] = &st
	d.stoppers[id] = routineStop
	d.nextID++
	d.mu.Unlock()
	return &st, nil
}

func (d *driver) ListPorts(ctx context.Context) ([]port.Status, error) {
	var ports []port.Status
	d.mu.Lock()
	for _, p := range d.ports {
		ports = append(ports, *p)
	}
	d.mu.Unlock()
	return ports, nil
}

func (d *driver) RemovePort(ctx context.Context, id int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	stop, ok := d.stoppers[id]
	if !ok {
		return errors.Errorf("unknown id: %d", id)
	}
	err := stop()
	delete(d.stoppers, id)
	delete(d.ports, id)
	return err
}
