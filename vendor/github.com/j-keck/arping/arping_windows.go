// windows currently not supported.
// dummy implementation to prevent compilation errors under windows

package arping

import (
	"errors"
	"net"
	"time"
)

var errWindowsNotSupported = errors.New("arping under windows not supported")

func initialize(iface net.Interface) error {
	return errWindowsNotSupported
}

func send(request arpDatagram) (time.Time, error) {
	return time.Now(), errWindowsNotSupported
}

func receive() (arpDatagram, time.Time, error) {
	return arpDatagram{}, time.Now(), errWindowsNotSupported
}

func deinitialize() error {
	return errWindowsNotSupported
}
