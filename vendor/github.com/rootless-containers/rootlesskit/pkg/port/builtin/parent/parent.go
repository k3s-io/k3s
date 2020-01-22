package parent

import (
	"context"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
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
