//go:build !windows
// +build !windows

package server

import (
	"fmt"
	"os"
)

// checkPermissions checks to see if the process is running as root
// Ref: https://github.com/kubernetes/kubernetes/pull/96616
func checkPermissions() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("server must run as root, or with --rootless and/or --disable-agent")
	}
	return nil
}
