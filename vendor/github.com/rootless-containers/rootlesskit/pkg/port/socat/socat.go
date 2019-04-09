package socat

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"

	"github.com/rootless-containers/rootlesskit/pkg/port"
	"github.com/rootless-containers/rootlesskit/pkg/port/portutil"
)

func NewParentDriver(logWriter io.Writer) (port.ParentDriver, error) {
	if _, err := exec.LookPath("socat"); err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("nsenter"); err != nil {
		return nil, err
	}
	d := driver{
		logWriter: logWriter,
		ports:     make(map[int]*port.Status, 0),
		stoppers:  make(map[int]func() error, 0),
		nextID:    1,
	}
	return &d, nil
}

type driver struct {
	logWriter io.Writer
	mu        sync.Mutex
	childPID  int
	ports     map[int]*port.Status
	stoppers  map[int]func() error
	nextID    int
}

func (d *driver) OpaqueForChild() map[string]string {
	// NOP, as this driver does not have child-side logic.
	return nil
}

func (d *driver) RunParentDriver(initComplete chan struct{}, quit <-chan struct{}, cctx *port.ChildContext) error {
	if cctx == nil || cctx.PID <= 0 {
		return errors.New("child PID not set")
	}
	d.childPID = cctx.PID
	initComplete <- struct{}{}
	<-quit
	return nil
}

func (d *driver) AddPort(ctx context.Context, spec port.Spec) (*port.Status, error) {
	if d.childPID <= 0 {
		return nil, errors.New("child PID not set")
	}
	d.mu.Lock()
	err := portutil.ValidatePortSpec(spec, d.ports)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	cf := func() (*exec.Cmd, error) {
		return createSocatCmd(ctx, spec, d.logWriter, d.childPID)
	}
	routineErrorCh := make(chan error)
	routineStopCh := make(chan struct{})
	routineStop := func() error {
		close(routineStopCh)
		return <-routineErrorCh
	}
	go portRoutine(cf, routineStopCh, routineErrorCh, d.logWriter)
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
		return errors.Errorf("unknown port id: %d", id)
	}
	err := stop()
	delete(d.stoppers, id)
	delete(d.ports, id)
	return err
}

func createSocatCmd(ctx context.Context, spec port.Spec, logWriter io.Writer, childPID int) (*exec.Cmd, error) {
	if spec.Proto != "tcp" && spec.Proto != "udp" {
		return nil, errors.Errorf("unsupported proto: %s", spec.Proto)
	}
	ipStr := "0.0.0.0"
	if spec.ParentIP != "" {
		ip := net.ParseIP(spec.ParentIP)
		if ip == nil {
			return nil, errors.Errorf("unsupported parentIP: %s", spec.ParentIP)
		}
		ip = ip.To4()
		if ip == nil {
			return nil, errors.Errorf("unsupported parentIP (v6?): %s", spec.ParentIP)
		}
		ipStr = ip.String()
	}
	if spec.ParentPort < 1 || spec.ParentPort > 65535 {
		return nil, errors.Errorf("unsupported parentPort: %d", spec.ParentPort)
	}
	if spec.ChildPort < 1 || spec.ChildPort > 65535 {
		return nil, errors.Errorf("unsupported childPort: %d", spec.ChildPort)
	}
	var cmd *exec.Cmd
	switch spec.Proto {
	case "tcp":
		cmd = exec.CommandContext(ctx,
			"socat",
			fmt.Sprintf("TCP-LISTEN:%d,bind=%s,reuseaddr,fork,rcvbuf=65536,sndbuf=65536", spec.ParentPort, ipStr),
			fmt.Sprintf("EXEC:\"%s\",nofork",
				fmt.Sprintf("nsenter -U -n --preserve-credentials -t %d socat STDIN TCP4:127.0.0.1:%d", childPID, spec.ChildPort)))
	case "udp":
		cmd = exec.CommandContext(ctx,
			"socat",
			fmt.Sprintf("UDP-LISTEN:%d,bind=%s,reuseaddr,fork,rcvbuf=65536,sndbuf=65536", spec.ParentPort, ipStr),
			fmt.Sprintf("EXEC:\"%s\",nofork",
				fmt.Sprintf("nsenter -U -n --preserve-credentials -t %d socat STDIN UDP4:127.0.0.1:%d", childPID, spec.ChildPort)))
	}
	cmd.Env = os.Environ()
	cmd.Stdout = logWriter
	cmd.Stderr = logWriter
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
	return cmd, nil
}

type cmdFactory func() (*exec.Cmd, error)

func portRoutine(cf cmdFactory, stopCh <-chan struct{}, errWCh chan error, logWriter io.Writer) {
	retry := 0
	doneCh := make(chan error)
	for {
		cmd, err := cf()
		if err != nil {
			errWCh <- err
			return
		}
		cmdDesc := fmt.Sprintf("%s %v", cmd.Path, cmd.Args)
		fmt.Fprintf(logWriter, "[exec] starting cmd %s\n", cmdDesc)
		if err := cmd.Start(); err != nil {
			errWCh <- err
			return
		}
		pid := cmd.Process.Pid
		go func() {
			err := cmd.Wait()
			doneCh <- err
		}()
		select {
		case err := <-doneCh:
			// even if err == nil (unexpected for socat), continue the loop
			retry++
			sleepDuration := time.Duration((retry*100)%(30*1000)) * time.Millisecond
			fmt.Fprintf(logWriter, "[exec] retrying cmd %s after sleeping %v, count=%d, err=%v\n",
				cmdDesc, sleepDuration, retry, err)
			select {
			case <-time.After(sleepDuration):
			case <-stopCh:
				errWCh <- err
				return
			}
		case <-stopCh:
			fmt.Fprintf(logWriter, "[exec] killing cmd %s pid %d\n", cmdDesc, pid)
			syscall.Kill(pid, syscall.SIGKILL)
			fmt.Fprintf(logWriter, "[exec] killed cmd %s pid %d\n", cmdDesc, pid)
			close(errWCh)
			return
		}
	}
}

func NewChildDriver() port.ChildDriver {
	return &childDriver{}
}

type childDriver struct {
}

func (d *childDriver) RunChildDriver(opaque map[string]string, quit <-chan struct{}) error {
	// NOP
	<-quit
	return nil
}
