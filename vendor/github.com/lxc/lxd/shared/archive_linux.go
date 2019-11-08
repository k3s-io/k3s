package shared

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/lxc/lxd/shared/ioprogress"
	"github.com/lxc/lxd/shared/logger"
)

func DetectCompression(fname string) ([]string, string, []string, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, "", nil, err
	}
	defer f.Close()

	return DetectCompressionFile(f)
}

func DetectCompressionFile(f io.ReadSeeker) ([]string, string, []string, error) {
	// read header parts to detect compression method
	// bz2 - 2 bytes, 'BZ' signature/magic number
	// gz - 2 bytes, 0x1f 0x8b
	// lzma - 6 bytes, { [0x000, 0xE0], '7', 'z', 'X', 'Z', 0x00 } -
	// xy - 6 bytes,  header format { 0xFD, '7', 'z', 'X', 'Z', 0x00 }
	// tar - 263 bytes, trying to get ustar from 257 - 262
	header := make([]byte, 263)
	_, err := f.Read(header)
	if err != nil {
		return nil, "", nil, err
	}

	switch {
	case bytes.Equal(header[0:2], []byte{'B', 'Z'}):
		return []string{"-jxf"}, ".tar.bz2", []string{"bzip2", "-d"}, nil
	case bytes.Equal(header[0:2], []byte{0x1f, 0x8b}):
		return []string{"-zxf"}, ".tar.gz", []string{"gzip", "-d"}, nil
	case (bytes.Equal(header[1:5], []byte{'7', 'z', 'X', 'Z'}) && header[0] == 0xFD):
		return []string{"-Jxf"}, ".tar.xz", []string{"xz", "-d"}, nil
	case (bytes.Equal(header[1:5], []byte{'7', 'z', 'X', 'Z'}) && header[0] != 0xFD):
		return []string{"--lzma", "-xf"}, ".tar.lzma", []string{"lzma", "-d"}, nil
	case bytes.Equal(header[0:3], []byte{0x5d, 0x00, 0x00}):
		return []string{"--lzma", "-xf"}, ".tar.lzma", []string{"lzma", "-d"}, nil
	case bytes.Equal(header[257:262], []byte{'u', 's', 't', 'a', 'r'}):
		return []string{"-xf"}, ".tar", []string{}, nil
	case bytes.Equal(header[0:4], []byte{'h', 's', 'q', 's'}):
		return []string{"-xf"}, ".squashfs",
			[]string{"sqfs2tar", "--no-skip"}, nil
	default:
		return nil, "", nil, fmt.Errorf("Unsupported compression")
	}
}

func Unpack(file string, path string, blockBackend bool, runningInUserns bool, tracker *ioprogress.ProgressTracker) error {
	extractArgs, extension, _, err := DetectCompression(file)
	if err != nil {
		return err
	}

	command := ""
	args := []string{}
	var reader io.Reader
	if strings.HasPrefix(extension, ".tar") {
		command = "tar"
		if runningInUserns {
			args = append(args, "--wildcards")
			args = append(args, "--exclude=dev/*")
			args = append(args, "--exclude=./dev/*")
			args = append(args, "--exclude=rootfs/dev/*")
			args = append(args, "--exclude=rootfs/./dev/*")
		}
		args = append(args, "-C", path, "--numeric-owner", "--xattrs-include=*")
		args = append(args, extractArgs...)
		args = append(args, "-")

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		reader = f

		// Attach the ProgressTracker if supplied.
		if tracker != nil {
			fsinfo, err := f.Stat()
			if err != nil {
				return err
			}

			tracker.Length = fsinfo.Size()
			reader = &ioprogress.ProgressReader{
				ReadCloser: f,
				Tracker:    tracker,
			}
		}
	} else if strings.HasPrefix(extension, ".squashfs") {
		// unsquashfs does not support reading from stdin,
		// so ProgressTracker is not possible.
		command = "unsquashfs"
		args = append(args, "-f", "-d", path, "-n")

		// Limit unsquashfs chunk size to 10% of memory and up to 256MB (default)
		// When running on a low memory system, also disable multi-processing
		mem, err := DeviceTotalMemory()
		mem = mem / 1024 / 1024 / 10
		if err == nil && mem < 256 {
			args = append(args, "-da", fmt.Sprintf("%d", mem), "-fr", fmt.Sprintf("%d", mem), "-p", "1")
		}

		args = append(args, file)
	} else {
		return fmt.Errorf("Unsupported image format: %s", extension)
	}

	err = RunCommandWithFds(reader, nil, command, args...)
	if err != nil {
		// Check if we ran out of space
		fs := unix.Statfs_t{}

		err1 := unix.Statfs(path, &fs)
		if err1 != nil {
			return err1
		}

		// Check if we're running out of space
		if int64(fs.Bfree) < int64(2*fs.Bsize) {
			if blockBackend {
				return fmt.Errorf("Unable to unpack image, run out of disk space (consider increasing your pool's volume.size)")
			} else {
				return fmt.Errorf("Unable to unpack image, run out of disk space")
			}
		}

		logger.Debugf("Unpacking failed")
		logger.Debugf(err.Error())
		return fmt.Errorf("Unpack failed, %s.", err)
	}

	return nil
}
