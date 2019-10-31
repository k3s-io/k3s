package pipe

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"

	"github.com/pkg/errors"
)

func ToHTTP(ctx context.Context, url string, tlsConfig *tls.Config) (net.Conn, error) {
	request, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}

	request = request.WithContext(ctx)
	netDial := &net.Dialer{}

	if deadline, ok := ctx.Deadline(); ok {
		netDial.Deadline = deadline
	}

	conn, err := tls.DialWithDialer(netDial, "tcp", request.URL.Host, tlsConfig)
	if err != nil {
		return nil, errors.Wrap(err, "tls dial")
	}

	err = request.Write(conn)
	if err != nil {
		return nil, errors.Wrap(err, "request write")
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return nil, errors.Wrap(err, "read request")
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("expected 101 response, got: %d", response.StatusCode)
	}

	listener, err := net.Listen("unix", "")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create unix listener")
	}
	defer listener.Close()

	if err := Unix(conn, listener.Addr().String()); err != nil {
		return nil, err
	}

	return listener.Accept()
}
