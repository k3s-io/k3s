// +build !windows

package containerd

import (
	"os/exec"
	"syscall"
)

func addDeathSig(cmd *exec.Cmd) {
	// not supported in this OS
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
	}
}
