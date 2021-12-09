//go:build !windows
// +build !windows

package wguser

import (
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
)

// dial is the default implementation of Client.dial.
func dial(device string) (net.Conn, error) {
	return net.Dial("unix", device)
}

// find is the default implementation of Client.find.
func find() ([]string, error) {
	return findUNIXSockets([]string{
		// It seems that /var/run is a common location between Linux and the
		// BSDs, even though it's a symlink on Linux.
		"/var/run/wireguard",
	})
}

// findUNIXSockets looks for UNIX socket files in the specified directories.
func findUNIXSockets(dirs []string) ([]string, error) {
	var socks []string
	for _, d := range dirs {
		files, err := ioutil.ReadDir(d)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return nil, err
		}

		for _, f := range files {
			if f.Mode()&os.ModeSocket == 0 {
				continue
			}

			socks = append(socks, filepath.Join(d, f.Name()))
		}
	}

	return socks, nil
}
