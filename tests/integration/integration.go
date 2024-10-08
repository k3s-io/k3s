package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/k3s-io/k3s/pkg/flock"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Compile-time variable
var existingServer = "False"

const lockFile = "/tmp/k3s-test.lock"

type K3sServer struct {
	cmd *exec.Cmd
	log *os.File
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
		logrus.Fatalf("Unable to find k3s executable in %s", k3sBin)
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
// Command output from both Stderr and Stdout is provided via string. Input can
// be a single string with space separated args, or multiple string args
// cmdEx1, err := K3sCmd("etcd-snapshot", "ls")
// cmdEx2, err := K3sCmd("kubectl get pods -A")
// cmdEx2, err := K3sCmd("kubectl", "get", "pods", "-A")
func K3sCmd(inputArgs ...string) (string, error) {
	if !IsRoot() {
		return "", errors.New("integration tests must be run as sudo/root")
	}
	k3sBin := findK3sExecutable()
	var k3sCmd []string
	for _, arg := range inputArgs {
		k3sCmd = append(k3sCmd, strings.Fields(arg)...)
	}
	cmd := exec.Command(k3sBin, k3sCmd...)
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

// K3sDefaultDeployments checks if the default deployments for K3s are ready, otherwise returns an error
func K3sDefaultDeployments() error {
	return CheckDeployments([]string{"coredns", "local-path-provisioner", "metrics-server", "traefik"})
}

// CheckDeployments checks if the provided list of deployments are ready, otherwise returns an error
func CheckDeployments(deployments []string) error {

	deploymentSet := make(map[string]bool)
	for _, d := range deployments {
		deploymentSet[d] = false
	}

	client, err := k8sClient()
	if err != nil {
		return err
	}
	deploymentList, err := client.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, deployment := range deploymentList.Items {
		if _, ok := deploymentSet[deployment.Name]; ok && deployment.Status.ReadyReplicas == deployment.Status.Replicas {
			deploymentSet[deployment.Name] = true
		}
	}
	for d, found := range deploymentSet {
		if !found {
			return fmt.Errorf("failed to deploy %s", d)
		}
	}

	return nil
}

func ParsePods(namespace string, opts metav1.ListOptions) ([]corev1.Pod, error) {
	clientSet, err := k8sClient()
	if err != nil {
		return nil, err
	}
	pods, err := clientSet.CoreV1().Pods(namespace).List(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func ParseNodes() ([]corev1.Node, error) {
	clientSet, err := k8sClient()
	if err != nil {
		return nil, err
	}
	nodes, err := clientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return nodes.Items, nil
}

func GetPod(namespace, name string) (*corev1.Pod, error) {
	client, err := k8sClient()
	if err != nil {
		return nil, err
	}
	return client.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func GetPersistentVolumeClaim(namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	client, err := k8sClient()
	if err != nil {
		return nil, err
	}
	return client.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), name, metav1.GetOptions{})
}

func GetPersistentVolume(name string) (*corev1.PersistentVolume, error) {
	client, err := k8sClient()
	if err != nil {
		return nil, err
	}
	return client.CoreV1().PersistentVolumes().Get(context.Background(), name, metav1.GetOptions{})
}

func SearchK3sLog(k3s *K3sServer, target string) (bool, error) {
	file, err := os.Open(k3s.log.Name())
	if err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), target) {
			return true, nil
		}
	}
	if scanner.Err() != nil {
		return false, scanner.Err()
	}
	return false, nil
}

func K3sTestLock() (int, error) {
	logrus.Info("waiting to get test lock")
	return flock.Acquire(lockFile)
}

// K3sStartServer acquires an exclusive lock on a temporary file, then launches a k3s cluster
// with the provided arguments. Subsequent/parallel calls to this function will block until
// the original lock is cleared using K3sKillServer
func K3sStartServer(inputArgs ...string) (*K3sServer, error) {
	if !IsRoot() {
		return nil, errors.New("integration tests must be run as sudo/root")
	}

	k3sBin := findK3sExecutable()
	k3sCmd := append([]string{"server"}, inputArgs...)
	cmd := exec.Command(k3sBin, k3sCmd...)
	// Give the server a new group id so we can kill it and its children later
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Pipe output to a file for debugging later
	f, err := os.Create("./k3log.txt")
	if err != nil {
		return nil, err
	}
	cmd.Stderr = f
	logrus.Info("Starting k3s server. Check k3log.txt for logs")
	err = cmd.Start()
	return &K3sServer{cmd, f}, err
}

// K3sStopServer gracefully stops the running K3s server and does not kill its children.
// Equivalent to stopping the K3s service
func K3sStopServer(server *K3sServer) error {
	if server.log != nil {
		server.log.Close()
	}
	if err := server.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	time.Sleep(10 * time.Second)
	if err := server.cmd.Process.Kill(); err != nil {
		return err
	}
	if _, err := server.cmd.Process.Wait(); err != nil {
		return err
	}
	return nil
}

// K3sKillServer terminates the running K3s server and its children.
// Equivalent to k3s-killall.sh
func K3sKillServer(server *K3sServer) error {
	if server == nil {
		return nil
	}
	if server.log != nil {
		server.log.Close()
		os.Remove(server.log.Name())
	}
	pgid, err := syscall.Getpgid(server.cmd.Process.Pid)
	if err != nil {
		if errors.Is(err, syscall.ESRCH) {
			logrus.Warnf("Unable to kill k3s server: %v", err)
			return nil
		}
		return errors.Wrap(err, "failed to find k3s process group")
	}
	if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
		return errors.Wrap(err, "failed to kill k3s process group")
	}
	if err := server.cmd.Process.Kill(); err != nil {
		return errors.Wrap(err, "failed to kill k3s process")
	}
	if _, err = server.cmd.Process.Wait(); err != nil {
		return errors.Wrap(err, "failed to wait for k3s process exit")
	}
	//Unmount all the associated filesystems
	unmountFolder("/run/k3s")
	unmountFolder("/run/netns/cni-")

	return nil
}

// K3sCleanup unlocks the test-lock and
// attempts to cleanup networking and files leftover from an integration test.
// This is similar to the k3s-killall.sh script, but we dynamically generate that on
// install, so we don't have access to it during testing.
func K3sCleanup(k3sTestLock int, dataDir string) error {
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
	if dataDir == "" {
		dataDir = "/var/lib/rancher/k3s"
	}
	if err := os.RemoveAll(dataDir); err != nil {
		return err
	}
	if k3sTestLock != -1 {
		return flock.Release(k3sTestLock)
	}
	return nil
}

func K3sSaveLog(server *K3sServer, dump bool) error {
	server.log.Close()
	if !dump {
		return nil
	}
	log, err := os.Open(server.log.Name())
	if err != nil {
		return err
	}
	defer log.Close()
	b, err := io.ReadAll(log)
	if err != nil {
		return err
	}
	fmt.Printf("Server Log Dump:\n\n%s\n\n", b)
	return nil
}

func GetEndpointsAddresses() (string, error) {
	client, err := k8sClient()
	if err != nil {
		return "", err
	}

	endpoints, err := client.CoreV1().Endpoints("default").Get(context.Background(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	var addresses []string
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			addresses = append(addresses, address.IP)
		}
	}

	return strings.Join(addresses, ","), nil
}

// RunCommand Runs command on the host
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	if err != nil {
		return out.String(), fmt.Errorf("%s", err)
	}
	return out.String(), nil
}

func unmountFolder(folder string) error {
	mounts := []string{}
	f, err := os.ReadFile("/proc/self/mounts")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(f), "\n") {
		columns := strings.Split(line, " ")
		if len(columns) > 1 {
			mounts = append(mounts, columns[1])
		}
	}

	for _, mount := range mounts {
		if strings.Contains(mount, folder) {
			if err := unix.Unmount(mount, 0); err != nil {
				return err
			}
			if err = os.Remove(mount); err != nil {
				return err
			}
		}
	}
	return nil
}

func k8sClient() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", "/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}
