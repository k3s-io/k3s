package child

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/rootless-containers/rootlesskit/pkg/msgutil"
	"github.com/rootless-containers/rootlesskit/pkg/port"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/msg"
	opaquepkg "github.com/rootless-containers/rootlesskit/pkg/port/builtin/opaque"
)

func NewDriver(logWriter io.Writer) port.ChildDriver {
	return &childDriver{
		logWriter: logWriter,
	}
}

type childDriver struct {
	logWriter io.Writer
}

func (d *childDriver) RunChildDriver(opaque map[string]string, quit <-chan struct{}) error {
	socketPath := opaque[opaquepkg.SocketPath]
	if socketPath == "" {
		return errors.New("socket path not set")
	}
	childReadyPipePath := opaque[opaquepkg.ChildReadyPipePath]
	if childReadyPipePath == "" {
		return errors.New("child ready pipe path not set")
	}
	childReadyPipeW, err := os.OpenFile(childReadyPipePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return err
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: socketPath,
		Net:  "unix",
	})
	if err != nil {
		return err
	}
	// write nothing, just close
	if err = childReadyPipeW.Close(); err != nil {
		return err
	}
	stopAccept := make(chan struct{}, 1)
	go func() {
		<-quit
		stopAccept <- struct{}{}
		ln.Close()
	}()
	for {
		c, err := ln.AcceptUnix()
		if err != nil {
			select {
			case <-stopAccept:
				return nil
			default:
			}
			return err
		}
		go func() {
			if rerr := d.routine(c); rerr != nil {
				rep := msg.Reply{
					Error: rerr.Error(),
				}
				msgutil.MarshalToWriter(c, &rep)
			}
			c.Close()
		}()
	}
	return nil
}

func (d *childDriver) routine(c *net.UnixConn) error {
	var req msg.Request
	if _, err := msgutil.UnmarshalFromReader(c, &req); err != nil {
		return err
	}
	switch req.Type {
	case msg.RequestTypeInit:
		return d.handleConnectInit(c, &req)
	case msg.RequestTypeConnect:
		return d.handleConnectRequest(c, &req)
	default:
		return errors.Errorf("unknown request type %q", req.Type)
	}
}

func (d *childDriver) handleConnectInit(c *net.UnixConn, req *msg.Request) error {
	_, err := msgutil.MarshalToWriter(c, nil)
	return err
}

func (d *childDriver) handleConnectRequest(c *net.UnixConn, req *msg.Request) error {
	switch req.Proto {
	case "tcp":
	case "udp":
	default:
		return errors.Errorf("unknown proto: %q", req.Proto)
	}
	var dialer net.Dialer
	targetConn, err := dialer.Dial(req.Proto, fmt.Sprintf("127.0.0.1:%d", req.Port))
	if err != nil {
		return err
	}
	defer targetConn.Close() // no effect on duplicated FD
	targetConnFiler, ok := targetConn.(filer)
	if !ok {
		return errors.Errorf("unknown target connection: %+v", targetConn)
	}
	targetConnFile, err := targetConnFiler.File()
	if err != nil {
		return err
	}
	defer targetConnFile.Close()
	oob := unix.UnixRights(int(targetConnFile.Fd()))
	f, err := c.File()
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		err = unix.Sendmsg(int(f.Fd()), []byte("dummy"), oob, nil, 0)
		if err != unix.EINTR {
			break
		}
	}
	return err
}

// filer is implemented by *net.TCPConn and *net.UDPConn
type filer interface {
	File() (f *os.File, err error)
}
