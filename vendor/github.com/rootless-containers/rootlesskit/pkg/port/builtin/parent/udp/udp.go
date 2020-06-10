package udp

import (
	"fmt"
	"io"
	"net"
	"os"

	"github.com/pkg/errors"

	"github.com/rootless-containers/rootlesskit/pkg/port"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/msg"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/parent/udp/udpproxy"
)

func Run(socketPath string, spec port.Spec, stopCh <-chan struct{}, logWriter io.Writer) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", spec.ParentIP, spec.ParentPort))
	if err != nil {
		return err
	}
	c, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	udpp := &udpproxy.UDPProxy{
		LogWriter: logWriter,
		Listener:  c,
		BackendDial: func() (*net.UDPConn, error) {
			// get fd from the child as an SCM_RIGHTS cmsg
			fd, err := msg.ConnectToChildWithRetry(socketPath, spec, 10)
			if err != nil {
				return nil, err
			}
			f := os.NewFile(uintptr(fd), "")
			defer f.Close()
			fc, err := net.FileConn(f)
			if err != nil {
				return nil, err
			}
			uc, ok := fc.(*net.UDPConn)
			if !ok {
				return nil, errors.Errorf("file conn doesn't implement *net.UDPConn: %+v", fc)
			}
			return uc, nil
		},
	}
	go udpp.Run()
	go func() {
		for {
			select {
			case <-stopCh:
				// udpp.Close closes ln as well
				udpp.Close()
				return
			}
		}
	}()
	// no wait
	return nil
}
