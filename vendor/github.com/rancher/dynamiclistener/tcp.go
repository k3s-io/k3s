package dynamiclistener

import (
	"fmt"
	"net"
	"reflect"
	"time"
)

func NewTCPListener(ip string, port int) (net.Listener, error) {
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return nil, err
	}

	tcpListener, ok := l.(*net.TCPListener)
	if !ok {
		return nil, fmt.Errorf("wrong listener type: %v", reflect.TypeOf(tcpListener))
	}

	return tcpKeepAliveListener{
		TCPListener: tcpListener,
	}, nil
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
