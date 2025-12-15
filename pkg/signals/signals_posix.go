//go:build !windows

package signals

import (
	"os"

	"golang.org/x/sys/unix"
)

var shutdownSignals = []os.Signal{unix.SIGINT, unix.SIGTERM}
