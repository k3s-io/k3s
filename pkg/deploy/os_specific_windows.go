package deploy

import (
	"strings"
)

func convertOsFileName(fileName string) string {
	return strings.ReplaceAll(fileName, "_windows", "")
}

func skipOsFileName(fileName string) bool {
	return strings.Contains(fileName, "_linux")
}
