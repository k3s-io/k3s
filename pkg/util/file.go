package util

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
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

// ReadFile waits for a file to exist, then returns its trimmed contents as a string
func ReadFile(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	var trimmed string
	return trimmed, wait.PollUntilContextTimeout(ctx, 2*time.Second, 4*time.Minute, true, func(ctx context.Context) (bool, error) {
		b, err := os.ReadFile(path)
		if err == nil {
			trimmed = strings.TrimSpace(string(b))
			return true, nil
		} else if os.IsNotExist(err) {
			logrus.Infof("Waiting for file %q to be created\n", path)
			return false, nil
		}
		return false, err
	})
}

// AtomicWrite firsts writes data to a temp file, then renames to the destination file.
// This ensures that the destination file is never partially written.
func AtomicWrite(fileName string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(fileName), filepath.Base(fileName)+".tmp")
	if err != nil {
		return err
	}
	tmpName := f.Name()
	defer os.Remove(tmpName)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, fileName)
}
