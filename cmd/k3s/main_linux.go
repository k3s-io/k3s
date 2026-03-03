//go:build linux

package main

import (
	"os"
	"syscall"

	"github.com/k3s-io/k3s/pkg/util/errors"
)

const programPostfix = ""

func runExec(cmd string, args []string, calledAsInternal bool) (err error) {
	if err := syscall.Exec(cmd, args, os.Environ()); err != nil {
		return errors.WithMessagef(err, "exec %s failed", cmd)
	}
	return nil
}
