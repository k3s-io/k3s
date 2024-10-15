 //go:build linux
// +build linux

package main

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
)

// runExec executes a command with the given arguments.
// It returns an error if the execution fails.
func runExec(cmd string, args []string, calledAsInternal bool) error {
	// Optionally check if the command is executable before executing
	if _, err := os.Stat(cmd); os.IsNotExist(err) {
		return errors.Wrapf(err, "command does not exist: %s", cmd)
	}

	if err := syscall.Exec(cmd, args, os.Environ()); err != nil {
		return errors.Wrapf(err, "exec %s failed", cmd)
	}
	return nil
}
