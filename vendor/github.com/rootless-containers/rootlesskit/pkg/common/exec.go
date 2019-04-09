package common

import (
	"io"
	"os/exec"
	"syscall"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func GetExecExitStatus(err error) (int, bool) {
	err = errors.Cause(err)
	if err == nil {
		return 0, false
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return 0, false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, false
	}
	return status.ExitStatus(), true
}

func Execs(o io.Writer, env []string, cmds [][]string) error {
	for _, cmd := range cmds {
		var args []string
		if len(cmd) > 1 {
			args = cmd[1:]
		}
		x := exec.Command(cmd[0], args...)
		x.Stdin = nil
		x.Stdout = o
		x.Stderr = o
		x.Env = env
		x.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGKILL,
		}
		logrus.Debugf("executing %v", cmd)
		if err := x.Run(); err != nil {
			return err
		}
	}
	return nil
}
