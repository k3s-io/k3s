// +build !windows

package copy

import (
	"os"
	"syscall"
	"testing"
)

func setup(m *testing.M) {
	os.MkdirAll("test/data.copy", os.ModePerm)
	os.Symlink("test/data/case01", "test/data/case03/case01")
	os.Chmod("test/data/case07/dir_0555", 0555)
	os.Chmod("test/data/case07/file_0444", 0444)
	syscall.Mkfifo("test/data/case11/foo/bar", 0555)
}
