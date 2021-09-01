// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"unsafe"
)

func unixgramSocketpair() (l, r *os.File, err error) {
	fd, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET, 0)
	if err != nil {
		return nil, nil, os.NewSyscallError("socketpair",
			err.(syscall.Errno))
	}
	l = os.NewFile(uintptr(fd[0]), "socketpair-half1")
	r = os.NewFile(uintptr(fd[1]), "socketpair-half2")
	return
}

// Create a FUSE FS on the specified mount point without using
// fusermount.
func mountDirect(mountPoint string, opts *MountOptions, ready chan<- error) (fd int, err error) {
	fd, err = syscall.Open("/dev/fuse", os.O_RDWR, 0) // use syscall.Open since we want an int fd
	if err != nil {
		return
	}

	// managed to open dev/fuse, attempt to mount
	source := opts.FsName
	if source == "" {
		source = opts.Name
	}

	var flags uintptr
	flags |= syscall.MS_NOSUID | syscall.MS_NODEV

	// some values we need to pass to mount, but override possible since opts.Options comes after
	var r = []string{
		fmt.Sprintf("fd=%d", fd),
		"rootmode=40000",
		"user_id=0",
		"group_id=0",
	}
	r = append(r, opts.Options...)

	if opts.AllowOther {
		r = append(r, "allow_other")
	}

	err = syscall.Mount(opts.FsName, mountPoint, "fuse."+opts.Name, opts.DirectMountFlags, strings.Join(r, ","))
	if err != nil {
		syscall.Close(fd)
		return
	}

	// success
	close(ready)
	return
}

// Create a FUSE FS on the specified mount point.  The returned
// mount point is always absolute.
func mount(mountPoint string, opts *MountOptions, ready chan<- error) (fd int, err error) {
	if opts.DirectMount {
		fd, err := mountDirect(mountPoint, opts, ready)
		if err == nil {
			return fd, nil
		} else if opts.Debug {
			log.Printf("mount: failed to do direct mount: %s", err)
		}
	}

	local, remote, err := unixgramSocketpair()
	if err != nil {
		return
	}

	defer local.Close()
	defer remote.Close()

	bin, err := fusermountBinary()
	if err != nil {
		return 0, err
	}

	cmd := []string{bin, mountPoint}
	if s := opts.optionsStrings(); len(s) > 0 {
		cmd = append(cmd, "-o", strings.Join(s, ","))
	}
	proc, err := os.StartProcess(bin,
		cmd,
		&os.ProcAttr{
			Env:   []string{"_FUSE_COMMFD=3"},
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr, remote}})

	if err != nil {
		return
	}

	w, err := proc.Wait()
	if err != nil {
		return
	}
	if !w.Success() {
		err = fmt.Errorf("fusermount exited with code %v\n", w.Sys())
		return
	}

	fd, err = getConnection(local)
	if err != nil {
		return -1, err
	}

	// golang sets CLOEXEC on file descriptors when they are
	// acquired through normal operations (e.g. open).
	// Buf for fd, we have to set CLOEXEC manually
	syscall.CloseOnExec(fd)

	close(ready)
	return fd, err
}

func unmount(mountPoint string, opts *MountOptions) (err error) {
	if opts.DirectMount {
		// Attempt to directly unmount, if fails fallback to fusermount method
		err := syscall.Unmount(mountPoint, 0)
		if err == nil {
			return nil
		}
	}

	bin, err := fusermountBinary()
	if err != nil {
		return err
	}
	errBuf := bytes.Buffer{}
	cmd := exec.Command(bin, "-u", mountPoint)
	cmd.Stderr = &errBuf
	err = cmd.Run()
	if errBuf.Len() > 0 {
		return fmt.Errorf("%s (code %v)\n",
			errBuf.String(), err)
	}
	return err
}

func getConnection(local *os.File) (int, error) {
	var data [4]byte
	control := make([]byte, 4*256)

	// n, oobn, recvflags, from, errno  - todo: error checking.
	_, oobn, _, _,
		err := syscall.Recvmsg(
		int(local.Fd()), data[:], control[:], 0)
	if err != nil {
		return 0, err
	}

	message := *(*syscall.Cmsghdr)(unsafe.Pointer(&control[0]))
	fd := *(*int32)(unsafe.Pointer(uintptr(unsafe.Pointer(&control[0])) + syscall.SizeofCmsghdr))

	if message.Type != 1 {
		return 0, fmt.Errorf("getConnection: recvmsg returned wrong control type: %d", message.Type)
	}
	if oobn <= syscall.SizeofCmsghdr {
		return 0, fmt.Errorf("getConnection: too short control message. Length: %d", oobn)
	}
	if fd < 0 {
		return 0, fmt.Errorf("getConnection: fd < 0: %d", fd)
	}
	return int(fd), nil
}

// lookPathFallback - search binary in PATH and, if that fails,
// in fallbackDir. This is useful if PATH is possible empty.
func lookPathFallback(file string, fallbackDir string) (string, error) {
	binPath, err := exec.LookPath(file)
	if err == nil {
		return binPath, nil
	}

	abs := path.Join(fallbackDir, file)
	return exec.LookPath(abs)
}

func fusermountBinary() (string, error) {
	return lookPathFallback("fusermount", "/bin")
}

func umountBinary() (string, error) {
	return lookPathFallback("umount", "/bin")
}
