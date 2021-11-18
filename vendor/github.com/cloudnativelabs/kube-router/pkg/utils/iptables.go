package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

var hasWait bool

func init() {
	path, err := exec.LookPath("iptables-restore")
	if err != nil {
		return
	}
	args := []string{"iptables-restore", "--help"}
	cmd := exec.Cmd{
		Path: path,
		Args: args,
	}
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	hasWait = strings.Contains(string(cmdOutput), "wait")
}

// SaveInto calls `iptables-save` for given table and stores result in a given buffer.
func SaveInto(table string, buffer *bytes.Buffer) error {
	path, err := exec.LookPath("iptables-save")
	if err != nil {
		return err
	}
	stderrBuffer := bytes.NewBuffer(nil)
	args := []string{"iptables-save", "-t", table}
	cmd := exec.Cmd{
		Path:   path,
		Args:   args,
		Stdout: buffer,
		Stderr: stderrBuffer,
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v (%s)", err, stderrBuffer)
	}
	return nil
}

// Restore runs `iptables-restore` passing data through []byte.
func Restore(table string, data []byte) error {
	path, err := exec.LookPath("iptables-restore")
	if err != nil {
		return err
	}
	var args []string
	if hasWait {
		args = []string{"iptables-restore", "--wait", "-T", table}
	} else {
		args = []string{"iptables-restore", "-T", table}
	}
	cmd := exec.Cmd{
		Path:  path,
		Args:  args,
		Stdin: bytes.NewBuffer(data),
	}
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v (%s)", err, b)
	}

	return nil
}

// AppendUnique ensures that rule is in chain only once in the buffer and that the occurrence is at the end of the buffer
func AppendUnique(buffer bytes.Buffer, chain string, rule []string) bytes.Buffer {
	var desiredBuffer bytes.Buffer

	// First we need to remove any previous instances of the rule that exist, so that we can be sure that our version
	// is unique and appended to the very end of the buffer
	rules := strings.Split(buffer.String(), "\n")
	if len(rules) > 0 && rules[len(rules)-1] == "" {
		rules = rules[:len(rules)-1]
	}
	for _, foundRule := range rules {
		if strings.Contains(foundRule, chain) {
			if strings.Contains(foundRule, strings.Join(rule, " ")) {
				continue
			}
		}
		desiredBuffer.WriteString(foundRule + "\n")
	}

	// Now append the rule that we wanted to be unique
	desiredBuffer = Append(desiredBuffer, chain, rule)
	return desiredBuffer
}

// Append appends rule to chain at the end of buffer
func Append(buffer bytes.Buffer, chain string, rule []string) bytes.Buffer {
	ruleStr := strings.Join(append(append([]string{"-A", chain}, rule...), "\n"), " ")
	buffer.WriteString(ruleStr)
	return buffer
}
