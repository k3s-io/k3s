//go:build !windows

package util

import (
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func SetFileModeForPath(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func SetFileModeForFile(file *os.File, mode os.FileMode) error {
	return file.Chmod(mode)
}

// ReadFile reads from a file
func ReadFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	for start := time.Now(); time.Since(start) < 4*time.Minute; {
		vpnBytes, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(vpnBytes)), nil
		} else if os.IsNotExist(err) {
			logrus.Infof("Waiting for %s to be available\n", path)
			time.Sleep(2 * time.Second)
		} else {
			return "", err
		}
	}

	return "", errors.New("Timeout while trying to read the file")
}
