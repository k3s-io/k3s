package proxy

import (
	"io"
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type proxy struct {
	lconn, rconn io.ReadWriteCloser
	done         bool
	errc         chan error
}

func Proxy(lconn, rconn net.Conn) error {
	p := &proxy{
		lconn: lconn,
		rconn: rconn,
		errc:  make(chan error),
	}

	defer p.rconn.Close()
	defer p.lconn.Close()
	go p.pipe(p.lconn, p.rconn)
	go p.pipe(p.rconn, p.lconn)
	return <-p.errc
}

func (p *proxy) err(err error) {
	if p.done {
		return
	}
	if !errors.Is(err, io.EOF) {
		logrus.Warnf("Proxy error: %v", err)
	}
	p.done = true
	p.errc <- err
}

func (p *proxy) pipe(src, dst io.ReadWriter) {
	buff := make([]byte, 1<<15)
	for {
		n, err := src.Read(buff)
		if err != nil {
			p.err(errors.Wrap(err, "read failed"))
			return
		}
		_, err = dst.Write(buff[:n])
		if err != nil {
			p.err(errors.Wrap(err, "write failed"))
			return
		}
	}

}
