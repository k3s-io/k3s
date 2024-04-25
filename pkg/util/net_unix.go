//go:build !windows
// +build !windows

package util

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// permitReuse enables port and address sharing on the socket
func permitReuse(network, addr string, conn syscall.RawConn) error {
	return conn.Control(func(fd uintptr) {
		syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	})
}
