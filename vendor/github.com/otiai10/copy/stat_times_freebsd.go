// +build freebsd

package copy

import (
	"os"
	"syscall"
	"time"
)

func getTimeSpec(info os.FileInfo) timespec {
	stat := info.Sys().(*syscall.Stat_t)
	times := timespec{
		Mtime: info.ModTime(),
		Atime: time.Unix(int64(stat.Atimespec.Sec), int64(stat.Atimespec.Nsec)),
		Ctime: time.Unix(int64(stat.Ctimespec.Sec), int64(stat.Ctimespec.Nsec)),
	}
	return times
}
