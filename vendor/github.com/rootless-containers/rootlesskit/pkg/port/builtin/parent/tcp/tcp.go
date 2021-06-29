package tcp

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/rootless-containers/rootlesskit/pkg/port"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/msg"
)

func Run(socketPath string, spec port.Spec, stopCh <-chan struct{}, logWriter io.Writer) error {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", spec.ParentIP, spec.ParentPort))
	if err != nil {
		fmt.Fprintf(logWriter, "listen: %v\n", err)
		return err
	}
	newConns := make(chan net.Conn)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				fmt.Fprintf(logWriter, "accept: %v\n", err)
				close(newConns)
				return
			}
			newConns <- c
		}
	}()
	go func() {
		defer ln.Close()
		for {
			select {
			case c, ok := <-newConns:
				if !ok {
					return
				}
				go func() {
					if err := copyConnToChild(c, socketPath, spec, stopCh); err != nil {
						fmt.Fprintf(logWriter, "copyConnToChild: %v\n", err)
						return
					}
				}()
			case <-stopCh:
				return
			}
		}
	}()
	// no wait
	return nil
}

func copyConnToChild(c net.Conn, socketPath string, spec port.Spec, stopCh <-chan struct{}) error {
	defer c.Close()
	// get fd from the child as an SCM_RIGHTS cmsg
	fd, err := msg.ConnectToChildWithRetry(socketPath, spec, 10)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), "")
	defer f.Close()
	fc, err := net.FileConn(f)
	if err != nil {
		return err
	}
	defer fc.Close()
	bicopy(c, fc, stopCh)
	return nil
}

// bicopy is based on libnetwork/cmd/proxy/tcp_proxy.go .
// NOTE: sendfile(2) cannot be used for sockets
func bicopy(x, y net.Conn, quit <-chan struct{}) {
	var wg sync.WaitGroup
	var broker = func(to, from net.Conn) {
		io.Copy(to, from)
		if fromTCP, ok := from.(*net.TCPConn); ok {
			fromTCP.CloseRead()
		}
		if toTCP, ok := to.(*net.TCPConn); ok {
			toTCP.CloseWrite()
		}
		wg.Done()
	}

	wg.Add(2)
	go broker(x, y)
	go broker(y, x)
	finish := make(chan struct{})
	go func() {
		wg.Wait()
		close(finish)
	}()

	select {
	case <-quit:
	case <-finish:
	}
	x.Close()
	y.Close()
	<-finish
}
