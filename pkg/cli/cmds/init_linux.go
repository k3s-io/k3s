// +build linux,cgo

package cmds

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/containerd/sys"
	"github.com/erikdubbelboer/gspt"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rootless-containers/rootlesskit/pkg/parent/cgrouputil"
)

// HandleInit takes care of things that need to be done when running as process 1, usually in a
// Docker container. This includes evacuating the root cgroup and reaping child pids.
func HandleInit() error {
	if os.Getpid() != 1 {
		return nil
	}

	if !sys.RunningInUserNS() {
		// The root cgroup has to be empty to enable subtree_control, so evacuate it by placing
		// ourselves in the init cgroup.
		if err := cgrouputil.EvacuateCgroup2("init"); err != nil {
			return errors.Wrap(err, "failed to evacuate root cgroup")
		}
	}

	pwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "failed to get working directory for init process")
	}

	go reapChildren()

	// fork the main process to do work so that this init process can handle reaping pids
	// without interfering with any other exec's that the rest of the codebase may do.
	var wstatus syscall.WaitStatus
	pattrs := &syscall.ProcAttr{
		Dir: pwd,
		Env: os.Environ(),
		Sys: &syscall.SysProcAttr{Setsid: true},
		Files: []uintptr{
			uintptr(syscall.Stdin),
			uintptr(syscall.Stdout),
			uintptr(syscall.Stderr),
		},
	}
	pid, err := syscall.ForkExec(os.Args[0], os.Args, pattrs)
	if err != nil {
		return errors.Wrap(err, "failed to fork/exec "+version.Program)
	}

	gspt.SetProcTitle(os.Args[0] + " init")
	// wait for main process to exit, and return its status when it does
	_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	for err == syscall.EINTR {
		_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	}
	os.Exit(wstatus.ExitStatus())
	return nil
}

//reapChildren calls Wait4 whenever SIGCHLD is received
func reapChildren() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGCHLD)
	for {
		select {
		case <-sigs:
		}
		for {
			var wstatus syscall.WaitStatus
			_, err := syscall.Wait4(-1, &wstatus, 0, nil)
			for err == syscall.EINTR {
				_, err = syscall.Wait4(-1, &wstatus, 0, nil)
			}
			if err == nil || err == syscall.ECHILD {
				break
			}
		}
	}
}
