package client

import (
	"context"
	"crypto/tls"
	"net"
	"strings"

	"github.com/canonical/go-dqlite/internal/protocol"
)

// DefaultDialFunc is the default dial function, which can handle plain TCP and
// Unix socket endpoints. You can customize it with WithDialFunc()
func DefaultDialFunc(ctx context.Context, address string) (net.Conn, error) {
	if strings.HasPrefix(address, "@") {
		return protocol.UnixDial(ctx, address)
	}
	return protocol.TCPDial(ctx, address)
}

// DialFuncWithTLS returns a dial function that uses TLS encryption.
//
// The given dial function will be used to establish the network connection,
// and the given TLS config will be used for encryption.
func DialFuncWithTLS(dial DialFunc, config *tls.Config) DialFunc {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		conn, err := dial(ctx, addr)
		if err != nil {
			return nil, err
		}
		return tls.Client(conn, config), nil
	}
}
