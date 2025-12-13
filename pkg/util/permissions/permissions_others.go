//go:build !windows

package permissions

import (
	"errors"
	"os"
)

// IsPrivileged returns an error if the process is not running as root.
// Ref: https://github.com/kubernetes/kubernetes/pull/96616
func IsPrivileged() error {
	if os.Getuid() != 0 {
		return errors.New("not running as root")
	}
	return nil
}
