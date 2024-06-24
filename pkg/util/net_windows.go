//go:build windows
// +build windows

package util

import "syscall"

// permitReuse is a no-op; port and address reuse is not supported on Windows
func permitReuse(network, addr string, conn syscall.RawConn) error {
	return nil
}
