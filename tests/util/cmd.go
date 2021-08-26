package util

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/rancher/k3s/pkg/flock"
	"github.com/sirupsen/logrus"
)

// Compile-time variable
var existingServer = "False"

func findK3sExecutable() string {
	// if running on an existing cluster, it maybe installed via k3s.service
	// or run manually from dist/artifacts/k3s
	if IsExistingServer() {
		k3sBin, err := exec.LookPath("k3s")
		if err == nil {
			return k3sBin
		}
	}
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

func IsExistingServer() bool {
	return existingServer == "True"
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

func contains(source []string, target string) bool {
	for _, s := range source {
		if s == target {
			return true
		}
	}
	return false
}

// ServerArgsPresent checks if the given arguments are found in the running k3s server
func ServerArgsPresent(neededArgs []string) bool {
	currentArgs := K3sServerArgs()
	for _, arg := range neededArgs {
		if !contains(currentArgs, arg) {
			return false
		}
	}
	return true
}

// K3sServerArgs returns the list of arguments that the k3s server launched with
func K3sServerArgs() []string {
	results, err := K3sCmd("kubectl", "get", "nodes", "-o", `jsonpath='{.items[0].metadata.annotations.k3s\.io/node-args}'`)
	if err != nil {
		return nil
	}
	res := strings.ReplaceAll(results, "'", "")
	var args []string
	if err := json.Unmarshal([]byte(res), &args); err != nil {
		logrus.Error(err)
		return nil
	}
	return args
}

func FindStringInCmdAsync(scanner *bufio.Scanner, target string) bool {
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), target) {
			return true
		}
	}
	return false
}

type K3sServer struct {
	cmd     *exec.Cmd
	scanner *bufio.Scanner
	lock    int
}

// K3sStartServer acquires an exclusive lock on a temporary file, then launches a k3s cluster
// with the provided arguments. Subsequent/parallel calls to this function will block until
// the original lock is cleared using K3sKillServer
func K3sStartServer(cmdArgs ...string) (*K3sServer, error) {
	logrus.Info("waiting to get server lock")
	k3sLock, err := flock.Acquire("/var/lock/k3s-test.lock")
	if err != nil {
		return nil, err
	}

	k3sBin := findK3sExecutable()
	var cmd *exec.Cmd
	if IsRoot() {
		k3sCmd := append([]string{"server"}, cmdArgs...)
		cmd = exec.Command(k3sBin, k3sCmd...)
	} else {
		k3sCmd := append([]string{k3sBin, "server"}, cmdArgs...)
		cmd = exec.Command("sudo", k3sCmd...)
	}
	cmdOut, _ := cmd.StderrPipe()
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	return &K3sServer{cmd, bufio.NewScanner(cmdOut), k3sLock}, err
}

// K3sKillServer terminates the running K3s server and unlocks the file for
// other tests
func K3sKillServer(server *K3sServer) error {
	if IsRoot() {
		if err := server.cmd.Process.Kill(); err != nil {
			return err
		}
	} else {
		// Since k3s was launched as sudo, we can't just kill the process
		killCmd := exec.Command("sudo", "pkill", "k3s")
		if err := killCmd.Run(); err != nil {
			return err
		}
	}
	return flock.Release(server.lock)
}
