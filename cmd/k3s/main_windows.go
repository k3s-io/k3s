//go:build windows
// +build windows

package main

import (
	"os"
	"os/exec"
)

const programPostfix = ".exe"

func runExec(cmd string, args []string, calledAsInternal bool) (err error) {
	// syscall.Exec: not supported by windows
	if calledAsInternal {
		args = args[1:]
	}
	cmdObj := exec.Command(cmd, args...)
	cmdObj.Stdout = os.Stdout
	cmdObj.Stderr = os.Stderr
	cmdObj.Stdin = os.Stdin
	cmdObj.Env = os.Environ()
	return cmdObj.Run()
}
