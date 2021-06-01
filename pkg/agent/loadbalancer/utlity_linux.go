// +build linux

package loadbalancer

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func reusePort(network, address string, conn syscall.RawConn) error {
	return conn.Control(func(descriptor uintptr) {
		syscall.SetsockoptInt(int(descriptor), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
}
