// +build windows

package shared

import (
	"os"
)

func GetOwnerMode(fInfo os.FileInfo) (os.FileMode, int, int) {
	return fInfo.Mode(), -1, -1
}
