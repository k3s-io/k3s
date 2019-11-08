package protocol

import (
	"context"
	"net"
)

// TCPDial is a dial function using plain TCP to establish the network
// connection.
func TCPDial(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "tcp", address)
}

// UnixDial is a dial function using Unix sockets to establish the network
// connection.
func UnixDial(ctx context.Context, address string) (net.Conn, error) {
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, "unix", address)
}
