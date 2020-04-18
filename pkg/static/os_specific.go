// +build !windows

package static

import (
	"strings"
)

func convertOsFileName(fileName string) string {
	return strings.ReplaceAll(fileName, "_linux", "")
}

func skipOsFileName(fileName string) bool {
	return strings.Contains(fileName, "_windows")
}
