// +build linux
// +build cgo

package shared

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/lxc/lxd/shared/logger"
)

/*
#ifndef _GNU_SOURCE
#define _GNU_SOURCE 1
#endif
#include <errno.h>
#include <fcntl.h>
#include <grp.h>
#include <limits.h>
#include <poll.h>
#include <pty.h>
#include <pwd.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/stat.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/un.h>

#define ABSTRACT_UNIX_SOCK_LEN sizeof(((struct sockaddr_un *)0)->sun_path)

// This is an adaption from https://codereview.appspot.com/4589049, to be
// included in the stdlib with the stdlib's license.

void configure_pty(int fd) {
	struct termios term_settings;
	struct winsize win;

	if (tcgetattr(fd, &term_settings) < 0) {
		fprintf(stderr, "Failed to get settings: %s\n", strerror(errno));
		return;
	}

	term_settings.c_iflag |= IMAXBEL;
	term_settings.c_iflag |= IUTF8;
	term_settings.c_iflag |= BRKINT;
	term_settings.c_iflag |= IXANY;

	term_settings.c_cflag |= HUPCL;

	if (tcsetattr(fd, TCSANOW, &term_settings) < 0) {
		fprintf(stderr, "Failed to set settings: %s\n", strerror(errno));
		return;
	}

	if (ioctl(fd, TIOCGWINSZ, &win) < 0) {
		fprintf(stderr, "Failed to get the terminal size: %s\n", strerror(errno));
		return;
	}

	win.ws_col = 80;
	win.ws_row = 25;

	if (ioctl(fd, TIOCSWINSZ, &win) < 0) {
		fprintf(stderr, "Failed to set the terminal size: %s\n", strerror(errno));
		return;
	}

	if (fcntl(fd, F_SETFD, FD_CLOEXEC) < 0) {
		fprintf(stderr, "Failed to set FD_CLOEXEC: %s\n", strerror(errno));
		return;
	}

	return;
}

void create_pty(int *master, int *slave, uid_t uid, gid_t gid) {
	if (openpty(master, slave, NULL, NULL, NULL) < 0) {
		fprintf(stderr, "Failed to openpty: %s\n", strerror(errno));
		return;
	}

	configure_pty(*master);
	configure_pty(*slave);

	if (fchown(*slave, uid, gid) < 0) {
		fprintf(stderr, "Warning: error chowning pty to container root\n");
		fprintf(stderr, "Continuing...\n");
	}
}

void create_pipe(int *master, int *slave) {
	int pipefd[2];

	if (pipe2(pipefd, O_CLOEXEC) < 0) {
		fprintf(stderr, "Failed to create a pipe: %s\n", strerror(errno));
		return;
	}

	*master = pipefd[0];
	*slave = pipefd[1];
}

int get_poll_revents(int lfd, int timeout, int flags, int *revents, int *saved_errno)
{
	int ret;
	struct pollfd pfd = {lfd, flags, 0};

again:
	ret = poll(&pfd, 1, timeout);
	if (ret < 0) {
		if (errno == EINTR)
			goto again;

		*saved_errno = errno;
		fprintf(stderr, "Failed to poll() on file descriptor.\n");
		return -1;
	}

	*revents = pfd.revents;

	return ret;
}
*/
import "C"

const ABSTRACT_UNIX_SOCK_LEN int = C.ABSTRACT_UNIX_SOCK_LEN

const POLLIN int = C.POLLIN
const POLLPRI int = C.POLLPRI
const POLLNVAL int = C.POLLNVAL
const POLLERR int = C.POLLERR
const POLLHUP int = C.POLLHUP
const POLLRDHUP int = C.POLLRDHUP

func GetPollRevents(fd int, timeout int, flags int) (int, int, error) {
	var err error
	revents := C.int(0)
	saved_errno := C.int(0)

	ret := C.get_poll_revents(C.int(fd), C.int(timeout), C.int(flags), &revents, &saved_errno)
	if int(ret) < 0 {
		err = unix.Errno(saved_errno)
	}

	return int(ret), int(revents), err
}

func OpenPty(uid, gid int64) (master *os.File, slave *os.File, err error) {
	fd_master := C.int(-1)
	fd_slave := C.int(-1)
	rootUid := C.uid_t(uid)
	rootGid := C.gid_t(gid)

	C.create_pty(&fd_master, &fd_slave, rootUid, rootGid)

	if fd_master == -1 || fd_slave == -1 {
		return nil, nil, errors.New("Failed to create a new pts pair")
	}

	master = os.NewFile(uintptr(fd_master), "master")
	slave = os.NewFile(uintptr(fd_slave), "slave")

	return master, slave, nil
}

func Pipe() (master *os.File, slave *os.File, err error) {
	fd_master := C.int(-1)
	fd_slave := C.int(-1)

	C.create_pipe(&fd_master, &fd_slave)

	if fd_master == -1 || fd_slave == -1 {
		return nil, nil, errors.New("Failed to create a new pipe")
	}

	master = os.NewFile(uintptr(fd_master), "master")
	slave = os.NewFile(uintptr(fd_slave), "slave")

	return master, slave, nil
}

// UserId is an adaption from https://codereview.appspot.com/4589049.
func UserId(name string) (int, error) {
	var pw C.struct_passwd
	var result *C.struct_passwd

	bufSize := C.sysconf(C._SC_GETPW_R_SIZE_MAX)
	if bufSize < 0 {
		bufSize = 4096
	}

	buf := C.malloc(C.size_t(bufSize))
	if buf == nil {
		return -1, fmt.Errorf("allocation failed")
	}
	defer C.free(buf)

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

again:
	rv, errno := C.getpwnam_r(cname,
		&pw,
		(*C.char)(buf),
		C.size_t(bufSize),
		&result)
	if rv < 0 {
		// OOM killer will take care of us if we end up doing this too
		// often.
		if errno == unix.ERANGE {
			bufSize *= 2
			tmp := C.realloc(buf, C.size_t(bufSize))
			if tmp == nil {
				return -1, fmt.Errorf("allocation failed")
			}
			buf = tmp
			goto again
		}
		return -1, fmt.Errorf("failed user lookup: %s", unix.Errno(rv))
	}

	if result == nil {
		return -1, fmt.Errorf("unknown user %s", name)
	}

	return int(C.int(result.pw_uid)), nil
}

// GroupId is an adaption from https://codereview.appspot.com/4589049.
func GroupId(name string) (int, error) {
	var grp C.struct_group
	var result *C.struct_group

	bufSize := C.sysconf(C._SC_GETGR_R_SIZE_MAX)
	if bufSize < 0 {
		bufSize = 4096
	}

	buf := C.malloc(C.size_t(bufSize))
	if buf == nil {
		return -1, fmt.Errorf("allocation failed")
	}

	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

again:
	rv, errno := C.getgrnam_r(cname,
		&grp,
		(*C.char)(buf),
		C.size_t(bufSize),
		&result)
	if rv != 0 {
		// OOM killer will take care of us if we end up doing this too
		// often.
		if errno == unix.ERANGE {
			bufSize *= 2
			tmp := C.realloc(buf, C.size_t(bufSize))
			if tmp == nil {
				return -1, fmt.Errorf("allocation failed")
			}
			buf = tmp
			goto again
		}

		C.free(buf)
		return -1, fmt.Errorf("failed group lookup: %s", unix.Errno(rv))
	}
	C.free(buf)

	if result == nil {
		return -1, fmt.Errorf("unknown group %s", name)
	}

	return int(C.int(result.gr_gid)), nil
}

// Extensively commented directly in the code. Please leave the comments!
// Looking at this in a couple of months noone will know why and how this works
// anymore.
func ExecReaderToChannel(r io.Reader, bufferSize int, exited <-chan bool, fd int) <-chan []byte {
	if bufferSize <= (128 * 1024) {
		bufferSize = (128 * 1024)
	}

	ch := make(chan ([]byte))

	// Takes care that the closeChannel() function is exactly executed once.
	// This allows us to avoid using a mutex.
	var once sync.Once
	closeChannel := func() {
		close(ch)
	}

	// [1]: This function has just one job: Dealing with the case where we
	// are running an interactive shell session where we put a process in
	// the background that does hold stdin/stdout open, but does not
	// generate any output at all. This case cannot be dealt with in the
	// following function call. Here's why: Assume the above case, now the
	// attached child (the shell in this example) exits. This will not
	// generate any poll() event: We won't get POLLHUP because the
	// background process is holding stdin/stdout open and noone is writing
	// to it. So we effectively block on GetPollRevents() in the function
	// below. Hence, we use another go routine here who's only job is to
	// handle that case: When we detect that the child has exited we check
	// whether a POLLIN or POLLHUP event has been generated. If not, we know
	// that there's nothing buffered on stdout and exit.
	var attachedChildIsDead int32 = 0
	go func() {
		<-exited

		atomic.StoreInt32(&attachedChildIsDead, 1)

		ret, revents, err := GetPollRevents(fd, 0, (POLLIN | POLLPRI | POLLERR | POLLHUP | POLLRDHUP | POLLNVAL))
		if ret < 0 {
			logger.Errorf("Failed to poll(POLLIN | POLLPRI | POLLHUP | POLLRDHUP) on file descriptor: %s.", err)
		} else if ret > 0 {
			if (revents & POLLERR) > 0 {
				logger.Warnf("Detected poll(POLLERR) event.")
			} else if (revents & POLLNVAL) > 0 {
				logger.Warnf("Detected poll(POLLNVAL) event.")
			}
		} else if ret == 0 {
			logger.Debugf("No data in stdout: exiting.")
			once.Do(closeChannel)
			return
		}
	}()

	go func() {
		readSize := (128 * 1024)
		offset := 0
		buf := make([]byte, bufferSize)
		avoidAtomicLoad := false

		defer once.Do(closeChannel)
		for {
			nr := 0
			var err error

			ret, revents, err := GetPollRevents(fd, -1, (POLLIN | POLLPRI | POLLERR | POLLHUP | POLLRDHUP | POLLNVAL))
			if ret < 0 {
				// This condition is only reached in cases where we are massively f*cked since we even handle
				// EINTR in the underlying C wrapper around poll(). So let's exit here.
				logger.Errorf("Failed to poll(POLLIN | POLLPRI | POLLERR | POLLHUP | POLLRDHUP) on file descriptor: %s. Exiting.", err)
				return
			}

			// [2]: If the process exits before all its data has been read by us and no other process holds stdin or
			// stdout open, then we will observe a (POLLHUP | POLLRDHUP | POLLIN) event. This means, we need to
			// keep on reading from the pty file descriptor until we get a simple POLLHUP back.
			both := ((revents & (POLLIN | POLLPRI)) > 0) && ((revents & (POLLHUP | POLLRDHUP)) > 0)
			if both {
				logger.Debugf("Detected poll(POLLIN | POLLPRI | POLLHUP | POLLRDHUP) event.")
				read := buf[offset : offset+readSize]
				nr, err = r.Read(read)
			}

			if (revents & POLLERR) > 0 {
				logger.Warnf("Detected poll(POLLERR) event: exiting.")
				return
			} else if (revents & POLLNVAL) > 0 {
				logger.Warnf("Detected poll(POLLNVAL) event: exiting.")
				return
			}

			if ((revents & (POLLIN | POLLPRI)) > 0) && !both {
				// This might appear unintuitive at first but is actually a nice trick: Assume we are running
				// a shell session in a container and put a process in the background that is writing to
				// stdout. Now assume the attached process (aka the shell in this example) exits because we
				// used Ctrl+D to send EOF or something. If no other process would be holding stdout open we
				// would expect to observe either a (POLLHUP | POLLRDHUP | POLLIN | POLLPRI) event if there
				// is still data buffered from the previous process or a simple (POLLHUP | POLLRDHUP) if
				// no data is buffered. The fact that we only observe a (POLLIN | POLLPRI) event means that
				// another process is holding stdout open and is writing to it.
				// One counter argument that can be leveraged is (brauner looks at tycho :))
				// "Hey, you need to write at least one additional tty buffer to make sure that
				// everything that the attached child has written is actually shown."
				// The answer to that is:
				// "This case can only happen if the process has exited and has left data in stdout which
				// would generate a (POLLIN | POLLPRI | POLLHUP | POLLRDHUP) event and this case is already
				// handled and triggers another codepath. (See [2].)"
				if avoidAtomicLoad || atomic.LoadInt32(&attachedChildIsDead) == 1 {
					avoidAtomicLoad = true
					// Handle race between atomic.StorInt32() in the go routine
					// explained in [1] and atomic.LoadInt32() in the go routine
					// here:
					// We need to check for (POLLHUP | POLLRDHUP) here again since we might
					// still be handling a pure POLLIN event from a write prior to the childs
					// exit. But the child might have exited right before and performed
					// atomic.StoreInt32() to update attachedChildIsDead before we
					// performed our atomic.LoadInt32(). This means we accidentally hit this
					// codepath and are misinformed about the available poll() events. So we
					// need to perform a non-blocking poll() again to exclude that case:
					//
					// - If we detect no (POLLHUP | POLLRDHUP) event we know the child
					//   has already exited but someone else is holding stdin/stdout open and
					//   writing to it.
					//   Note that his case should only ever be triggered in situations like
					//   running a shell and doing stuff like:
					//    > ./lxc exec xen1 -- bash
					//   root@xen1:~# yes &
					//   .
					//   .
					//   .
					//   now send Ctrl+D or type "exit". By the time the Ctrl+D/exit event is
					//   triggered, we will have read all of the childs data it has written to
					//   stdout and so we can assume that anything that comes now belongs to
					//   the process that is holding stdin/stdout open.
					//
					// - If we detect a (POLLHUP | POLLRDHUP) event we know that we've
					//   hit this codepath on accident caused by the race between
					//   atomic.StoreInt32() in the go routine explained in [1] and
					//   atomic.LoadInt32() in this go routine. So the next call to
					//   GetPollRevents() will either return
					//   (POLLIN | POLLPRI | POLLERR | POLLHUP | POLLRDHUP)
					//   or (POLLHUP | POLLRDHUP). Both will trigger another codepath (See [2].)
					//   that takes care that all data of the child that is buffered in
					//   stdout is written out.
					ret, revents, err := GetPollRevents(fd, 0, (POLLIN | POLLPRI | POLLERR | POLLHUP | POLLRDHUP | POLLNVAL))
					if ret < 0 {
						logger.Errorf("Failed to poll(POLLIN | POLLPRI | POLLERR | POLLHUP | POLLRDHUP) on file descriptor: %s. Exiting.", err)
						return
					} else if (revents & (POLLHUP | POLLRDHUP | POLLERR | POLLNVAL)) == 0 {
						logger.Debugf("Exiting but background processes are still running.")
						return
					}
				}
				read := buf[offset : offset+readSize]
				nr, err = r.Read(read)
			}

			// The attached process has exited and we have read all data that may have
			// been buffered.
			if ((revents & (POLLHUP | POLLRDHUP)) > 0) && !both {
				logger.Debugf("Detected poll(POLLHUP) event: exiting.")
				return
			}

			offset += nr
			if offset > 0 && (offset+readSize >= bufferSize || err != nil) {
				ch <- buf[0:offset]
				offset = 0
				buf = make([]byte, bufferSize)
			}
		}
	}()

	return ch
}
