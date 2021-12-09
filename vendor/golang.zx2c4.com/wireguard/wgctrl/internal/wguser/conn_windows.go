//go:build windows
// +build windows

package wguser

import (
	"net"
	"strings"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/ipc/winpipe"
)

// Expected prefixes when dealing with named pipes.
const (
	pipePrefix = `\\.\pipe\`
	wgPrefix   = `ProtectedPrefix\Administrators\WireGuard\`
)

// dial is the default implementation of Client.dial.
func dial(device string) (net.Conn, error) {
	localSystem, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return nil, err
	}

	pipeCfg := &winpipe.DialConfig{
		ExpectedOwner: localSystem,
	}
	return winpipe.Dial(device, nil, pipeCfg)
}

// find is the default implementation of Client.find.
func find() ([]string, error) {
	return findNamedPipes(wgPrefix)
}

// findNamedPipes looks for Windows named pipes that match the specified
// search string prefix.
func findNamedPipes(search string) ([]string, error) {
	var (
		pipes []string
		data  windows.Win32finddata
	)

	// Thanks @zx2c4 for the tips on the appropriate Windows APIs here:
	// https://א.cc/dHGpnhxX/c.
	h, err := windows.FindFirstFile(
		// Append * to find all named pipes.
		windows.StringToUTF16Ptr(pipePrefix+"*"),
		&data,
	)
	if err != nil {
		return nil, err
	}

	// FindClose is used to close file search handles instead of the typical
	// CloseHandle used elsewhere, see:
	// https://docs.microsoft.com/en-us/windows/desktop/api/fileapi/nf-fileapi-findclose.
	defer windows.FindClose(h)

	// Check the first file's name for a match, but also keep searching for
	// WireGuard named pipes until no more files can be iterated.
	for {
		name := windows.UTF16ToString(data.FileName[:])
		if strings.HasPrefix(name, search) {
			// Concatenate strings directly as filepath.Join appears to break the
			// named pipe prefix convention.
			pipes = append(pipes, pipePrefix+name)
		}

		if err := windows.FindNextFile(h, &data); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}

			return nil, err
		}
	}

	return pipes, nil
}
