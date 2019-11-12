package dqlite

import (
	"context"
	"net"
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/dqlite/pipe"
)

var (
	upgradeResponse = []byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: dqlite\r\n\r\n")
)

type proxy struct {
	conns chan net.Conn
}

func newProxy(ctx context.Context, bindAddress string) http.Handler {
	p := &proxy{
		conns: make(chan net.Conn, 100),
	}
	go func() {
		<-ctx.Done()
		close(p.conns)
	}()
	go pipe.UnixPiper(p.conns, bindAddress)

	return p
}

func (h *proxy) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	hijacker, ok := rw.(http.Hijacker)
	if !ok {
		http.Error(rw, "failed to hijack", http.StatusInternalServerError)
		return
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		err := errors.Wrap(err, "Hijack connection")
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	if n, err := conn.Write(upgradeResponse); err != nil || n != len(upgradeResponse) {
		conn.Close()
		return
	}

	h.conns <- conn
}
