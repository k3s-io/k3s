package protocol

import (
	"context"
	"crypto/tls"
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

// TLSCipherSuites are the cipher suites by the go-dqlite TLS helpers.
var TLSCipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
}
