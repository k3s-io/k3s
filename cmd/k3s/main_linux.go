//go:build linux
// +build linux

package main

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
)

const programPostfix = ""

func runExec(cmd string, args []string, calledAsInternal bool) (err error) {
	if err := syscall.Exec(cmd, args, os.Environ()); err != nil {
		return errors.Wrapf(err, "exec %s failed", cmd)
	}
	return nil
}
