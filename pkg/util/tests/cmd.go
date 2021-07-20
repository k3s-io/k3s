package tests

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

func IsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	return currentUser.Username == "root"
}

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

//Launch a k3s command asynchronously
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
	err := cmd.Start()
	return cmd, bufio.NewScanner(cmdOut), err
}

func K3sKillAsync(cmd *exec.Cmd) error {
	if IsRoot() {
		return cmd.Process.Kill()
	}
	// Since k3s was launched as sudo, we can't just kill the process
	killCmd := exec.Command("sudo", "pkill", "k3s")
	return killCmd.Run()
}
