package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"

	"github.com/rancher/k3s/pkg/flock"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// Compile-time variable
var existingServer = "False"

const lockFile = "/var/lock/k3s-test.lock"

type K3sServer struct {
	cmd     *exec.Cmd
	scanner *bufio.Scanner
	lock    int
}

type Node struct {
	Name       string
	Status     string
	Roles      string
	InternalIP string
	ExternalIP string
}

type Pod struct {
	NameSpace string
	Name      string
	Ready     string
	Status    string
	Restarts  string
	NodeIP    string
	Node      string
}

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
	i := 0
	for ; i < 20; i++ {
		_, err := os.Stat(k3sBin)
		if err != nil {
			k3sBin = "../" + k3sBin
			continue
		}
		break
	}
	if i == 20 {
		logrus.Fatal("Unable to find k3s executable")
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

// RunCommand launches the provided K3s command via exec. Command blocks until finished.
// Command output from both Stderr and Stdout is provided via string.
//   cmdEx1, err := RunCommand("etcd-snapshot", "ls")
//   cmdEx2, err := RunCommand("kubectl", "get", "pods", "-A")
func RunCommand(cmdName string, cmdArgs ...string) (string, error) {
	if !IsRoot() {
		return "", fmt.Errorf("integration tests must be run as sudo/root")
	}
	k3sBin := findK3sExecutable()
	// Only run sudo if not root
	k3sCmd := append([]string{cmdName}, cmdArgs...)
	cmd := exec.Command(k3sBin, k3sCmd...)
	byteOut, err := cmd.CombinedOutput()
	return string(byteOut), err
}

func ParsePods(kubeconfig string, debug bool) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	podList := ""

	cmd := "kubectl get pods -o wide --no-headers -A --kubeconfig=" + kubeconfig
	res, _ := RunCommand(cmd)
	res = strings.TrimSpace(res)
	podList = res

	split := strings.Split(res, "\n")
	for _, rec := range split {
		fields := strings.Fields(string(rec))
		pod := Pod{
			NameSpace: fields[0],
			Name:      fields[1],
			Ready:     fields[2],
			Status:    fields[3],
			Restarts:  fields[4],
			NodeIP:    fields[6],
			Node:      fields[7],
		}
		pods = append(pods, pod)
	}
	if debug {
		fmt.Println(podList)
	}
	return pods, nil
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
	results, err := RunCommand("kubectl", "get", "nodes", "-o", `jsonpath='{.items[0].metadata.annotations.k3s\.io/node-args}'`)
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

// K3sStartServer acquires an exclusive lock on a temporary file, then launches a k3s cluster
// with the provided arguments. Subsequent/parallel calls to this function will block until
// the original lock is cleared using K3sKillServer
func K3sStartServer(inputArgs ...string) (*K3sServer, error) {
	if !IsRoot() {
		return nil, fmt.Errorf("integration tests must be run as sudo/root")
	}

	logrus.Info("waiting to get server lock")
	k3sLock, err := flock.Acquire(lockFile)
	if err != nil {
		return nil, err
	}

	var cmdArgs []string
	for _, arg := range inputArgs {
		cmdArgs = append(cmdArgs, strings.Fields(arg)...)
	}

	k3sBin := findK3sExecutable()

	k3sCmd := append([]string{"server"}, cmdArgs...)
	cmd := exec.Command(k3sBin, k3sCmd...)
	// Give the server a new group id so we can kill it and its children later
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmdOut, _ := cmd.StderrPipe()
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	return &K3sServer{cmd, bufio.NewScanner(cmdOut), k3sLock}, err
}

// K3sKillServer terminates the running K3s server and its children
// and unlocks the file for other tests
func K3sKillServer(server *K3sServer, releaseLock bool) error {
	pgid, err := syscall.Getpgid(server.cmd.Process.Pid)
	if err != nil {
		return err
	}
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		return err
	}
	if releaseLock {
		return flock.Release(server.lock)
	}
	return nil
}

// K3sCleanup attempts to cleanup networking and files leftover from an integration test
func K3sCleanup(server *K3sServer, releaseLock bool) error {
	if cni0Link, err := netlink.LinkByName("cni0"); err == nil {
		links, _ := netlink.LinkList()
		for _, link := range links {
			if link.Attrs().MasterIndex == cni0Link.Attrs().Index {
				netlink.LinkDel(link)
			}
		}
		netlink.LinkDel(cni0Link)
	}

	if flannel1, err := netlink.LinkByName("flannel.1"); err == nil {
		netlink.LinkDel(flannel1)
	}
	if flannelV6, err := netlink.LinkByName("flannel-v6.1"); err == nil {
		netlink.LinkDel(flannelV6)
	}
	if err := os.RemoveAll("/var/lib/rancher/k3s"); err != nil {
		return err
	}
	if releaseLock {
		return flock.Release(server.lock)
	}
	return nil
}
