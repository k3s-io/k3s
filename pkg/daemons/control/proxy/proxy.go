package proxy

import (
	"io"
	"net"
	"syscall"

	"github.com/k3s-io/k3s/pkg/util/errors"

	"github.com/sirupsen/logrus"
)

type proxy struct {
	lconn, rconn io.ReadWriteCloser
	errc         chan error
}

func Proxy(lconn, rconn io.ReadWriteCloser) error {
	p := &proxy{
		lconn: lconn,
		rconn: rconn,
		errc:  make(chan error, 2), // one send per pipe goroutine, so neither can block
	}

	defer p.rconn.Close()
	defer p.lconn.Close()
	go p.pipe(p.lconn, p.rconn)
	go p.pipe(p.rconn, p.lconn)
	err := <-p.errc
	if !isConnTeardown(err) {
		logrus.Warnf("Proxy error: %v", err)
	}
	return err
}

// isConnTeardown reports errors that are expected when one side closes or resets the connection.
func isConnTeardown(err error) bool {
	return errors.Is(err, io.EOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}

func (p *proxy) pipe(src, dst io.ReadWriter) {
	buff := make([]byte, 1<<15)
	for {
		n, err := src.Read(buff)
		if err != nil {
			p.errc <- err
			return
		}
		_, err = dst.Write(buff[:n])
		if err != nil {
			p.errc <- err
			return
		}
	}
}
