package util

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func SetFileModeForPath(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func SetFileGroupForPath(name string, group string) error {
	// Try to use as group id
	gid, err := strconv.Atoi(group)
	if err == nil {
		return os.Chown(name, -1, gid)
	}

	// Otherwise, it must be a group name
	g, err := user.LookupGroup(group)
	if err != nil {
		return err
	}

	gid, err = strconv.Atoi(g.Gid)
	if err != nil {
		return err
	}

	return os.Chown(name, -1, gid)
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
