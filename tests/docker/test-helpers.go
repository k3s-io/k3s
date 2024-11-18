package docker

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type TestConfig struct {
	TestDir        string
	KubeconfigFile string
	Label          string
	Secret         string
	K3sImage       string
	NumServers     int
	NumAgents      int
	Servers        []ServerConfig
	Agents         []AgentConfig
}

type ServerConfig struct {
	Name string
	Port int
	IP   string
	URL  string
}

type AgentConfig struct {
	Name string
	IP   string
}

// NewTestConfig initializes the test environment and returns the configuration
// k3s version and tag information is extracted from the version.sh script
// and supplied as an argument to the function/test
func NewTestConfig(k3sImage string) (*TestConfig, error) {
	config := &TestConfig{
		K3sImage: k3sImage,
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "k3s-test-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	config.TestDir = tempDir
	// Setup cleanup on exit
	// setupCleanup(config)

	// Create required directories
	if err := os.MkdirAll(filepath.Join(config.TestDir, "logs"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Generate random secret
	config.Secret = fmt.Sprintf("%012d", rand.Intn(1000000000000))
	return config, nil
}

// portUsed checks if a port is in use
func portFree(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// getPort finds an available port
func getPort() int {
	var port int
	for i := 0; i < 100; i++ {
		port = 10000 + rand.Intn(50000)
		if portFree(port) {
			return port
		}
	}
	return -1
}

// ProvisionServers starts the required number of k3s servers
// and updates the kubeconfig file with the first server details
func (config *TestConfig) ProvisionServers(numOfServers int) error {
	config.NumServers = numOfServers
	for i := 0; i < config.NumServers; i++ {

		testID := filepath.Base(config.TestDir)
		name := fmt.Sprintf("k3s-server-%d-%s", i, strings.ToLower(testID))

		port := getPort()
		if port == -1 {
			return fmt.Errorf("failed to find an available port")
		}

		serverImage := getEnvOrDefault("K3S_IMAGE_SERVER", config.K3sImage)

		// Assemble all the Docker args
		dRun := strings.Join([]string{"docker run -d",
			"--name", name,
			"--hostname", name,
			"--privileged",
			"-p", fmt.Sprintf("127.0.0.1:%d:6443", port),
			"-p", "6443",
			"-e", fmt.Sprintf("K3S_TOKEN=%s", config.Secret),
			"-e", "K3S_DEBUG=true",
			os.Getenv("SERVER_DOCKER_ARGS"),
			os.Getenv(fmt.Sprintf("SERVER_%d_DOCKER_ARGS", i)),
			os.Getenv("REGISTRY_CLUSTER_ARGS"),
			serverImage,
			"server", os.Getenv("ARGS"), os.Getenv("SERVER_ARGS"), os.Getenv(fmt.Sprintf("SERVER_%d_ARGS", i))}, " ")
		if out, err := RunCommand(dRun); err != nil {
			return fmt.Errorf("failed to run server container: %s: %v", out, err)
		}

		// Get the IP address of the container
		ipOutput, err := RunCommand("docker inspect --format \"{{ .NetworkSettings.IPAddress }}\" " + name)
		if err != nil {
			return err
		}
		ip := strings.TrimSpace(ipOutput)

		url := fmt.Sprintf("https://%s:6443", ip)

		config.Servers = append(config.Servers, ServerConfig{
			Name: name,
			Port: port,
			IP:   ip,
			URL:  url,
		})

		fmt.Printf("Started %s @ %s\n", name, url)

	}

	// Wait for kubeconfig to be available
	time.Sleep(5 * time.Second)
	// Write kubeconfig from first serRver
	return copyAndModifyKubeconfig(config)
}

func (config *TestConfig) ProvisionAgents(numOfAgents int) error {
	config.NumAgents = numOfAgents
	if err := checkVersionSkew(config); err != nil {
		return err
	}
	testID := filepath.Base(config.TestDir)
	k3sURL := getEnvOrDefault("K3S_URL", config.Servers[0].URL)

	var g errgroup.Group
	for i := 0; i < config.NumAgents; i++ {
		i := i // capture loop variable
		g.Go(func() error {
			name := fmt.Sprintf("k3s-agent-%d-%s", i, strings.ToLower(testID))

			agentInstanceArgs := fmt.Sprintf("AGENT_%d_ARGS", i)

			// Assemble all the Docker args
			dRun := strings.Join([]string{"docker run -d",
				"--name", name,
				"--hostname", name,
				"--privileged",
				"-e", fmt.Sprintf("K3S_TOKEN=%s", config.Secret),
				"-e", fmt.Sprintf("K3S_URL=%s", k3sURL),
				os.Getenv("AGENT_DOCKER_ARGS"),
				os.Getenv(fmt.Sprintf("AGENT_%d_DOCKER_ARGS", i)),
				os.Getenv("REGISTRY_CLUSTER_ARGS"),
				getEnvOrDefault("K3S_IMAGE_AGENT", config.K3sImage),
				"agent", os.Getenv("ARGS"), os.Getenv("AGENT_ARGS"), os.Getenv(agentInstanceArgs)}, " ")

			if out, err := RunCommand(dRun); err != nil {
				return fmt.Errorf("failed to run agent container: %s: %v", out, err)
			}

			// Get the IP address of the container
			ipOutput, err := RunCommand("docker inspect --format \"{{ .NetworkSettings.IPAddress }}\" " + name)
			if err != nil {
				return err
			}
			ip := strings.TrimSpace(ipOutput)

			config.Agents = append(config.Agents, AgentConfig{
				Name: name,
				IP:   ip,
			})
			fmt.Printf("Started %s\n", name)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (config *TestConfig) Cleanup() error {

	errs := make([]error, 0)
	// Stop and remove all servers
	for _, server := range config.Servers {
		cmd := fmt.Sprintf("docker stop %s", server.Name)
		if _, err := RunCommand(cmd); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop server %s: %v", server.Name, err))
		}
		cmd = fmt.Sprintf("docker rm %s", server.Name)
		if _, err := RunCommand(cmd); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove server %s: %v", server.Name, err))
		}
	}

	// Stop and remove all agents
	for _, agent := range config.Agents {
		cmd := fmt.Sprintf("docker stop %s", agent.Name)
		if _, err := RunCommand(cmd); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop agent %s: %v", agent.Name, err))
		}
		cmd = fmt.Sprintf("docker rm %s", agent.Name)
		if _, err := RunCommand(cmd); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove agent %s: %v", agent.Name, err))
		}
	}

	// Error out if we hit any issues
	if len(errs) > 0 {
		return fmt.Errorf("cleanup failed: %v", errs)
	}

	if config.TestDir != "" {
		return os.RemoveAll(config.TestDir)
	}
	return nil
}

func copyAndModifyKubeconfig(config *TestConfig) error {
	if len(config.Servers) == 0 {
		return fmt.Errorf("no servers available to copy kubeconfig")
	}
	cmd := fmt.Sprintf("docker cp %s:/etc/rancher/k3s/k3s.yaml %s/kubeconfig.yaml", config.Servers[0].Name, config.TestDir)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to copy kubeconfig: %v", err)
	}

	cmd = fmt.Sprintf("sed -i -e \"s/:6443/:%d/g\" %s/kubeconfig.yaml", config.Servers[0].Port, config.TestDir)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to update kubeconfig: %v", err)
	}
	config.KubeconfigFile = filepath.Join(config.TestDir, "kubeconfig.yaml")
	fmt.Println("Kubeconfig file: ", config.KubeconfigFile)
	return nil
}

// PodReady checks if a pod is ready by querying its status
func PodReady(podName string, kubeconfig string) (bool, error) {

	cmd := fmt.Sprintf("kubectl get pods -n kube-system -o jsonpath='{.items[*].status.containerStatuses[?(@.name==\"%s\")].ready}' --kubeconfig=%s", podName, kubeconfig)
	output, err := RunCommand(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to get pod status: %v", err)
	}
	return strings.Contains(output, "true"), nil
}

// RunCmdOnDocker runs a command on a docker container
func RunCmdOnDocker(container, cmd string) (string, error) {
	dCmd := fmt.Sprintf("docker exec %s %s", container, cmd)
	return RunCommand(dCmd)
}

// RunCommand Runs command on the host.
// Returns stdout and embeds stderr inside the error message.
func RunCommand(cmd string) (string, error) {
	var stdout, stderr bytes.Buffer
	c := exec.Command("bash", "-c", cmd)
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("failed to run command: %s: %s: %v", cmd, stderr.String(), err)
	}
	return stdout.String(), nil
}

func checkVersionSkew(config *TestConfig) error {
	if config.NumAgents > 0 {
		serverImage := getEnvOrDefault("K3S_IMAGE_SERVER", config.K3sImage)
		agentImage := getEnvOrDefault("K3S_IMAGE_AGENT", config.K3sImage)
		if semver.Compare(semver.MajorMinor(agentImage), semver.MajorMinor(serverImage)) > 0 {
			return fmt.Errorf("agent version cannot be higher than server - not supported by Kubernetes version skew policy")
		}
	}

	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// VerifyValidVersion checks for invalid version strings
func VerifyValidVersion(container string, binary string) error {
	output, err := RunCmdOnDocker(container, binary+" version")
	if err != nil {
		return err
	}
	lines := strings.Split(output, "\n")
	// Check for invalid version strings
	re := regexp.MustCompile(`(?i).*(dev|head|unknown|fail|refuse|\+[^"]*\.).*`)
	for _, line := range lines {
		if re.MatchString(line) {
			return fmt.Errorf("invalid version string found in %s: %s", binary, line)
		}
	}

	return nil
}

// Returns the latest version from the update channel
func GetVersionFromChannel(upgradeChannel string) (string, error) {
	url := fmt.Sprintf("https://update.k3s.io/v1-release/channels/%s", upgradeChannel)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	finalURL := resp.Header.Get("Location")
	if finalURL == "" {
		return "", fmt.Errorf("location header not set")
	}
	version := finalURL[strings.LastIndex(finalURL, "/")+1:]
	version = strings.Replace(version, "+", "-", 1)
	return version, nil
}

// TODO the below functions are duplicated in the integration test utils. Consider combining into commmon package
// DeploymentsReady checks if the provided list of deployments are ready, otherwise returns an error
func DeploymentsReady(deployments []string, kubeconfigFile string) error {

	deploymentSet := make(map[string]bool)
	for _, d := range deployments {
		deploymentSet[d] = false
	}

	client, err := k8sClient(kubeconfigFile)
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

func ParseNodes(kubeconfigFile string) ([]corev1.Node, error) {
	clientSet, err := k8sClient(kubeconfigFile)
	if err != nil {
		return nil, err
	}
	nodes, err := clientSet.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return nodes.Items, nil
}

// Checks if all nodes are ready, otherwise returns an error
func NodesReady(kubeconfigFile string) error {
	nodes, err := ParseNodes(kubeconfigFile)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status != corev1.ConditionTrue {
				return fmt.Errorf("node %s is not ready", node.Name)
			}
		}
	}
	return nil
}

func k8sClient(kubeconfigFile string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile)
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}
