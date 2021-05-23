// +build !windows

package util

import (
	"os"
)

func SetFileModeForPath(name string, mode os.FileMode) error {
	fi, err := os.Stat(name)
	if err != nil {
		return err
	}
	if fi.Mode().Perm() != mode.Perm() {
		return os.Chmod(name, mode)
	}
	return nil
}

func SetFileModeForFile(file *os.File, mode os.FileMode) error {
	return file.Chmod(mode)
}
