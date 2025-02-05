package docker

import (
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
)

type TestConfig struct {
	TestDir        string
	KubeconfigFile string
	Token          string
	K3sImage       string
	DBType         string
	Servers        []Server
	Agents         []DockerNode
	ServerYaml     string
	AgentYaml      string
}

type DockerNode struct {
	Name string
	IP   string
}

type Server struct {
	DockerNode
	Port int
	URL  string
}

// NewTestConfig initializes the test environment and returns the configuration
// If k3sImage == "rancher/systemd-node", then the systemd-node container and the local k3s binary
// will be used to start the server. This is useful for scenarios where the server needs to be restarted.
// k3s version and tag information should be extracted from the version.sh script
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

	// Create required directories
	if err := os.MkdirAll(filepath.Join(config.TestDir, "logs"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Generate random secret
	config.Token = fmt.Sprintf("%012d", rand.Int63n(1000000000000))
	return config, nil
}

// portFree checks if a port is in use and returns true if it is free
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
// and updates the kubeconfig file with the first cp server details
func (config *TestConfig) ProvisionServers(numOfServers int) error {
	for i := 0; i < numOfServers; i++ {

		// If a server i already exists, skip. This is useful for scenarios where
		// the first server is started seperate from the rest of the servers
		if config.Servers != nil && i < len(config.Servers) {
			continue
		}

		testID := filepath.Base(config.TestDir)
		name := fmt.Sprintf("server-%d-%s", i, strings.ToLower(testID))

		port := getPort()
		if port == -1 {
			return fmt.Errorf("failed to find an available port")
		}

		// Write the server yaml to a tmp file and mount it into the container
		var yamlMount string
		if config.ServerYaml != "" {
			if err := os.WriteFile(filepath.Join(config.TestDir, fmt.Sprintf("server-%d.yaml", i)), []byte(config.ServerYaml), 0644); err != nil {
				return fmt.Errorf("failed to write server yaml: %v", err)
			}
			yamlMount = fmt.Sprintf("--mount type=bind,src=%s,dst=/etc/rancher/k3s/config.yaml", filepath.Join(config.TestDir, fmt.Sprintf("server-%d.yaml", i)))
		}

		var joinServer string
		var dbConnect string
		var err error
		if config.DBType == "" && numOfServers > 1 {
			config.DBType = "etcd"
		} else if config.DBType == "" {
			config.DBType = "sqlite"
		}
		if i == 0 {
			dbConnect, err = config.setupDatabase(true)
			if err != nil {
				return err
			}
		} else {
			dbConnect, err = config.setupDatabase(false)
			if err != nil {
				return err
			}
			if config.Servers[0].URL == "" {
				return fmt.Errorf("first server URL is empty")
			}
			joinServer = fmt.Sprintf("--server %s", config.Servers[0].URL)
		}
		newServer := Server{
			DockerNode: DockerNode{
				Name: name,
			},
			Port: port,
		}

		// If we need restarts, we use the systemd-node container, volume mount the k3s binary
		// and start the server using the install script
		if config.K3sImage == "rancher/systemd-node" {
			dRun := strings.Join([]string{"docker run -d",
				"--name", name,
				"--hostname", name,
				"--privileged",
				"-p", fmt.Sprintf("127.0.0.1:%d:6443", port),
				"--memory", "2048m",
				"-e", fmt.Sprintf("K3S_TOKEN=%s", config.Token),
				"-e", "K3S_DEBUG=true",
				"-e", "GOCOVERDIR=/tmp/k3s-cov",
				"-v", "/sys/fs/bpf:/sys/fs/bpf",
				"-v", "/lib/modules:/lib/modules",
				"-v", "/var/run/docker.sock:/var/run/docker.sock",
				"-v", "/var/lib/docker:/var/lib/docker",
				yamlMount,
				"--mount", "type=bind,source=$(pwd)/../../../dist/artifacts/k3s,target=/usr/local/bin/k3s",
				fmt.Sprintf("%s:v0.0.5", config.K3sImage),
				"/usr/lib/systemd/systemd --unit=noop.target --show-status=true"}, " ")
			if out, err := RunCommand(dRun); err != nil {
				return fmt.Errorf("failed to start systemd container: %s: %v", out, err)
			}
			time.Sleep(5 * time.Second)
			cmd := "mkdir -p /tmp/k3s-cov"
			if out, err := newServer.RunCmdOnNode(cmd); err != nil {
				return fmt.Errorf("failed to create coverage directory: %s: %v", out, err)
			}
			// The pipe requires that we use sh -c with "" to run the command
			cmd = fmt.Sprintf("/bin/sh -c \"curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='%s' INSTALL_K3S_SKIP_DOWNLOAD=true sh -\"",
				dbConnect+" "+joinServer+" "+os.Getenv(fmt.Sprintf("SERVER_%d_ARGS", i)))
			if out, err := newServer.RunCmdOnNode(cmd); err != nil {
				return fmt.Errorf("failed to start server: %s: %v", out, err)
			}
		} else {
			// Assemble all the Docker args
			dRun := strings.Join([]string{"docker run -d",
				"--name", name,
				"--hostname", name,
				"--privileged",
				"-p", fmt.Sprintf("127.0.0.1:%d:6443", port),
				"-p", "6443",
				"-e", fmt.Sprintf("K3S_TOKEN=%s", config.Token),
				"-e", "K3S_DEBUG=true",
				"-e", "GOCOVERDIR=/tmp/",
				os.Getenv("SERVER_DOCKER_ARGS"),
				os.Getenv(fmt.Sprintf("SERVER_%d_DOCKER_ARGS", i)),
				os.Getenv("REGISTRY_CLUSTER_ARGS"),
				yamlMount,
				config.K3sImage,
				"server", dbConnect, joinServer, os.Getenv(fmt.Sprintf("SERVER_%d_ARGS", i))}, " ")
			if out, err := RunCommand(dRun); err != nil {
				return fmt.Errorf("failed to run server container: %s: %v", out, err)
			}
		}

		// Get the IP address of the container
		ipOutput, err := RunCommand("docker inspect --format \"{{ .NetworkSettings.IPAddress }}\" " + name)
		if err != nil {
			return err
		}
		ip := strings.TrimSpace(ipOutput)

		url := fmt.Sprintf("https://%s:6443", ip)
		newServer.URL = url
		newServer.IP = ip
		config.Servers = append(config.Servers, newServer)

		fmt.Printf("Started %s @ %s\n", name, url)

		// Sleep for a bit to allow the first server to start
		if i == 0 && numOfServers > 1 {
			time.Sleep(10 * time.Second)
		}
	}

	// Wait for kubeconfig to be available
	time.Sleep(5 * time.Second)
	return copyAndModifyKubeconfig(config)
}

// setupDatabase will start the configured database if startDB is true,
// and return the correct flag to join the configured database
func (config *TestConfig) setupDatabase(startDB bool) (string, error) {

	joinFlag := ""
	startCmd := ""
	switch config.DBType {
	case "mysql":
		startCmd = "docker run -d --name mysql -e MYSQL_ROOT_PASSWORD=docker -p 3306:3306 mysql:8.4"
		joinFlag = "--datastore-endpoint='mysql://root:docker@tcp(172.17.0.1:3306)/k3s'"
	case "postgres":
		startCmd = "docker run -d --name postgres -e POSTGRES_PASSWORD=docker -p 5432:5432 postgres:16-alpine"
		joinFlag = "--datastore-endpoint='postgres://postgres:docker@tcp(172.17.0.1:5432)/k3s'"
	case "etcd":
		if startDB {
			joinFlag = "--cluster-init"
		}
	case "sqlite":
		break
	default:
		return "", fmt.Errorf("unsupported database type: %s", config.DBType)
	}

	if startDB && startCmd != "" {
		if out, err := RunCommand(startCmd); err != nil {
			return "", fmt.Errorf("failed to start %s container: %s: %v", config.DBType, out, err)
		}
		// Wait for DB to start
		time.Sleep(10 * time.Second)
	}
	return joinFlag, nil

}

func (config *TestConfig) ProvisionAgents(numOfAgents int) error {
	if err := checkVersionSkew(config); err != nil {
		return err
	}
	testID := filepath.Base(config.TestDir)
	k3sURL := getEnvOrDefault("K3S_URL", config.Servers[0].URL)

	var g errgroup.Group
	for i := 0; i < numOfAgents; i++ {
		i := i // capture loop variable
		g.Go(func() error {
			name := fmt.Sprintf("agent-%d-%s", i, strings.ToLower(testID))

			agentInstanceArgs := fmt.Sprintf("AGENT_%d_ARGS", i)
			newAgent := DockerNode{
				Name: name,
			}

			if config.K3sImage == "rancher/systemd-node" {
				dRun := strings.Join([]string{"docker run -d",
					"--name", name,
					"--hostname", name,
					"--privileged",
					"--memory", "2048m",
					"-e", fmt.Sprintf("K3S_TOKEN=%s", config.Token),
					"-e", fmt.Sprintf("K3S_URL=%s", k3sURL),
					"-v", "/sys/fs/bpf:/sys/fs/bpf",
					"-v", "/lib/modules:/lib/modules",
					"-v", "/var/run/docker.sock:/var/run/docker.sock",
					"-v", "/var/lib/docker:/var/lib/docker",
					"--mount", "type=bind,source=$(pwd)/../../../dist/artifacts/k3s,target=/usr/local/bin/k3s",
					fmt.Sprintf("%s:v0.0.5", config.K3sImage),
					"/usr/lib/systemd/systemd --unit=noop.target --show-status=true"}, " ")
				if out, err := RunCommand(dRun); err != nil {
					return fmt.Errorf("failed to start systemd container: %s: %v", out, err)
				}
				time.Sleep(5 * time.Second)
				// The pipe requires that we use sh -c with "" to run the command
				sCmd := fmt.Sprintf("/bin/sh -c \"curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='agent %s' INSTALL_K3S_SKIP_DOWNLOAD=true sh -\"",
					os.Getenv(agentInstanceArgs))

				if out, err := newAgent.RunCmdOnNode(sCmd); err != nil {
					return fmt.Errorf("failed to start server: %s: %v", out, err)
				}
			} else {
				// Assemble all the Docker args
				dRun := strings.Join([]string{"docker run -d",
					"--name", name,
					"--hostname", name,
					"--privileged",
					"-e", fmt.Sprintf("K3S_TOKEN=%s", config.Token),
					"-e", fmt.Sprintf("K3S_URL=%s", k3sURL),
					"-e", "GOCOVERDIR=/tmp/",
					os.Getenv("AGENT_DOCKER_ARGS"),
					os.Getenv(fmt.Sprintf("AGENT_%d_DOCKER_ARGS", i)),
					os.Getenv("REGISTRY_CLUSTER_ARGS"),
					config.K3sImage,
					"agent", os.Getenv("ARGS"), os.Getenv(agentInstanceArgs)}, " ")

				if out, err := RunCommand(dRun); err != nil {
					return fmt.Errorf("failed to run agent container: %s: %v", out, err)
				}
			}

			// Get the IP address of the container
			ipOutput, err := RunCommand("docker inspect --format \"{{ .NetworkSettings.IPAddress }}\" " + name)
			if err != nil {
				return err
			}
			ip := strings.TrimSpace(ipOutput)
			newAgent.IP = ip
			config.Agents = append(config.Agents, newAgent)

			fmt.Printf("Started %s\n", name)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (config *TestConfig) RemoveNode(nodeName string) error {
	cmd := fmt.Sprintf("docker stop %s", nodeName)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to stop node %s: %v", nodeName, err)
	}
	cmd = fmt.Sprintf("docker rm %s", nodeName)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to remove node %s: %v", nodeName, err)
	}
	return nil
}

// Returns a list of all node names
func (config *TestConfig) GetNodeNames() []string {
	var nodeNames []string
	for _, server := range config.Servers {
		nodeNames = append(nodeNames, server.Name)
	}
	for _, agent := range config.Agents {
		nodeNames = append(nodeNames, agent.Name)
	}
	return nodeNames
}

func (config *TestConfig) Cleanup() error {

	errs := make([]error, 0)
	// Stop and remove all servers
	for _, server := range config.Servers {
		if err := config.RemoveNode(server.Name); err != nil {
			errs = append(errs, err)
		}
	}

	// Stop and remove all agents
	for _, agent := range config.Agents {
		if err := config.RemoveNode(agent.Name); err != nil {
			errs = append(errs, err)
		}
	}

	// Stop DB if it was started
	if config.DBType == "mysql" || config.DBType == "postgres" {
		cmd := fmt.Sprintf("docker stop %s", config.DBType)
		if _, err := RunCommand(cmd); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop %s: %v", config.DBType, err))
		}
		cmd = fmt.Sprintf("docker rm %s", config.DBType)
		if _, err := RunCommand(cmd); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove %s: %v", config.DBType, err))
		}
	}

	// Error out if we hit any issues
	if len(errs) > 0 {
		return fmt.Errorf("cleanup failed: %v", errs)
	}

	if config.TestDir != "" {
		return os.RemoveAll(config.TestDir)
	}
	config.Agents = nil
	config.Servers = nil
	return nil
}

// copyAndModifyKubeconfig copies out kubeconfig from first control-plane server
// and updates the port to match the external port
func copyAndModifyKubeconfig(config *TestConfig) error {
	if len(config.Servers) == 0 {
		return fmt.Errorf("no servers available to copy kubeconfig")
	}

	serverID := 0
	for i := range config.Servers {
		server_args := os.Getenv(fmt.Sprintf("SERVER_%d_ARGS", i))
		if !strings.Contains(server_args, "--disable-apiserver") {
			serverID = i
			break
		}
	}

	cmd := fmt.Sprintf("docker cp %s:/etc/rancher/k3s/k3s.yaml %s/kubeconfig.yaml", config.Servers[serverID].Name, config.TestDir)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to copy kubeconfig: %v", err)
	}

	cmd = fmt.Sprintf("sed -i -e \"s/:6443/:%d/g\" %s/kubeconfig.yaml", config.Servers[serverID].Port, config.TestDir)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to update kubeconfig: %v", err)
	}
	config.KubeconfigFile = filepath.Join(config.TestDir, "kubeconfig.yaml")
	fmt.Println("Kubeconfig file: ", config.KubeconfigFile)
	return nil
}

// RunCmdOnNode runs a command on a docker container
func (node DockerNode) RunCmdOnNode(cmd string) (string, error) {
	dCmd := fmt.Sprintf("docker exec %s %s", node.Name, cmd)
	out, err := RunCommand(dCmd)
	if err != nil {
		return out, fmt.Errorf("%v: on node %s: %s", err, node.Name, out)
	}
	return out, nil
}

// RunCommand Runs command on the host.
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("failed to run command: %s, %v", cmd, err)
	}
	return string(out), err
}

func checkVersionSkew(config *TestConfig) error {
	if len(config.Agents) > 0 {
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
func VerifyValidVersion(node Server, binary string) error {
	output, err := node.RunCmdOnNode(binary + " version")
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

// TODO the below functions are replicated from e2e test utils. Consider combining into commmon package
func (config TestConfig) DeployWorkload(workload string) (string, error) {
	resourceDir := "../resources"
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}
	fmt.Println("\nDeploying", workload)
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl apply -f " + filename + " --kubeconfig=" + config.KubeconfigFile
			return RunCommand(cmd)
		}
	}
	return "", nil
}
