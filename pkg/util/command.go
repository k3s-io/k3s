package util

import (
	"bytes"
	"os/exec"
)

// ExecCommand executes a command using the VPN binary
// In case of error != nil, the string returned var will have more information
func ExecCommand(command string, args []string) (string, error) {
	var out, errOut bytes.Buffer

	cmd := exec.Command(command, args...)
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err != nil {
		return errOut.String(), err
	}
	return out.String(), nil
}
