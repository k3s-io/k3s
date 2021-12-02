// +build windows

package copy

import (
	"os"
)

// pcopy is for just named pipes. Windows doesn't support them
func pcopy(dest string, info os.FileInfo) error {
	return nil
}
