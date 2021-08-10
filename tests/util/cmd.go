package util

import (
	"bufio"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

func findK3sExecutable() string {
	k3sBin := "dist/artifacts/k3s"
	for {
		_, err := os.Stat(k3sBin)
		if err != nil {
			k3sBin = "../" + k3sBin
			continue
		}
		break
	}
	return k3sBin
}

// IsRoot return true if the user is root (UID 0)
func IsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	return currentUser.Uid == "0"
}

// K3sCmd launches the provided K3s command via exec. Command blocks until finished.
// Command output from both Stderr and Stdout is provided via string.
//   cmdEx1, err := K3sCmd("etcd-snapshot", "ls")
//   cmdEx2, err := K3sCmd("kubectl", "get", "pods", "-A")
func K3sCmd(cmdName string, cmdArgs ...string) (string, error) {
	k3sBin := findK3sExecutable()
	// Only run sudo if not root
	var cmd *exec.Cmd
	if IsRoot() {
		k3sCmd := append([]string{cmdName}, cmdArgs...)
		cmd = exec.Command(k3sBin, k3sCmd...)
	} else {
		k3sCmd := append([]string{k3sBin, cmdName}, cmdArgs...)
		cmd = exec.Command("sudo", k3sCmd...)
	}
	byteOut, err := cmd.CombinedOutput()
	return string(byteOut), err
}

func FindStringInCmdAsync(scanner *bufio.Scanner, target string) bool {
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), target) {
			return true
		}
	}
	return false
}

// K3sCmdAsync launches a k3s command asynchronously. Output of the command can be retrieved via
// the provided scanner. The command can be ended via cmd.Wait() or via K3sKillAsync()
func K3sCmdAsync(cmdName string, cmdArgs ...string) (*exec.Cmd, *bufio.Scanner, error) {
	k3sBin := findK3sExecutable()
	var cmd *exec.Cmd
	if IsRoot() {
		k3sCmd := append([]string{cmdName}, cmdArgs...)
		cmd = exec.Command(k3sBin, k3sCmd...)
	} else {
		k3sCmd := append([]string{k3sBin, cmdName}, cmdArgs...)
		cmd = exec.Command("sudo", k3sCmd...)
	}
	cmdOut, _ := cmd.StderrPipe()
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	return cmd, bufio.NewScanner(cmdOut), err
}

// K3sKillAsync terminates a command started by K3sCmdAsync(). This is
func K3sKillAsync(cmd *exec.Cmd) error {
	if IsRoot() {
		return cmd.Process.Kill()
	}
	// Since k3s was launched as sudo, we can't just kill the process
	killCmd := exec.Command("sudo", "pkill", "k3s")
	return killCmd.Run()
}
