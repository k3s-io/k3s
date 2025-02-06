package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	json "github.com/json-iterator/go"
	ginkgo "github.com/onsi/ginkgo/v2"
	"golang.org/x/sync/errgroup"
)

// defining the VagrantNode type allows methods like RunCmdOnNode to be defined on it.
// This makes test code more consistent, as similar functions can exists in Docker and E2E tests.
type VagrantNode string

func (v VagrantNode) String() string {
	return string(v)
}

func VagrantSlice(v []VagrantNode) []string {
	nodes := make([]string, 0, len(v))
	for _, node := range v {
		nodes = append(nodes, node.String())
	}
	return nodes
}

type TestConfig struct {
	Hardened       bool
	KubeconfigFile string
	Servers        []VagrantNode
	Agents         []VagrantNode
}

func (tc *TestConfig) Status() string {
	sN := strings.Join(VagrantSlice(tc.Servers), " ")
	aN := strings.Join(VagrantSlice(tc.Agents), " ")
	hardened := ""
	if tc.Hardened {
		hardened = "Hardened: true\n"
	}
	return fmt.Sprintf("%sKubeconfig: %s\nServers Nodes: %s\nAgents Nodes: %s\n)", hardened, tc.KubeconfigFile, sN, aN)
}

type Node struct {
	Name       string
	Status     string
	Roles      string
	InternalIP string
	ExternalIP string
}

func (n Node) String() string {
	return fmt.Sprintf("Node (name: %s, status: %s, roles: %s)", n.Name, n.Status, n.Roles)
}

type NodeError struct {
	Node VagrantNode
	Cmd  string
	Err  error
}

type SvcExternalIP struct {
	IP     string `json:"ip"`
	IPMode string `json:"ipMode"`
}

type ObjIP struct {
	Name string
	IPv4 string
	IPv6 string
}

func (ne *NodeError) Error() string {
	return fmt.Sprintf("failed creating cluster: %s: %v", ne.Cmd, ne.Err)
}

func (ne *NodeError) Unwrap() error {
	return ne.Err
}

func newNodeError(cmd string, node VagrantNode, err error) *NodeError {
	return &NodeError{
		Cmd:  cmd,
		Node: node,
		Err:  err,
	}
}

// genNodeEnvs generates the node and testing environment variables for vagrant up
func genNodeEnvs(nodeOS string, serverCount, agentCount int) ([]VagrantNode, []VagrantNode, string) {
	serverNodes := make([]VagrantNode, serverCount)
	for i := 0; i < serverCount; i++ {
		serverNodes[i] = VagrantNode("server-" + strconv.Itoa(i))
	}
	agentNodes := make([]VagrantNode, agentCount)
	for i := 0; i < agentCount; i++ {
		agentNodes[i] = VagrantNode("agent-" + strconv.Itoa(i))
	}

	nodeRoles := strings.Join(VagrantSlice(serverNodes), " ") + " " + strings.Join(VagrantSlice(agentNodes), " ")
	nodeRoles = strings.TrimSpace(nodeRoles)

	nodeBoxes := strings.Repeat(nodeOS+" ", serverCount+agentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)

	nodeEnvs := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s"`, nodeRoles, nodeBoxes)

	return serverNodes, agentNodes, nodeEnvs
}

func CreateCluster(nodeOS string, serverCount, agentCount int) (*TestConfig, error) {

	serverNodes, agentNodes, nodeEnvs := genNodeEnvs(nodeOS, serverCount, agentCount)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}
	// Bring up the first server node
	cmd := fmt.Sprintf(`%s %s vagrant up %s &> vagrant.log`, nodeEnvs, testOptions, serverNodes[0])
	fmt.Println(cmd)
	if _, err := RunCommand(cmd); err != nil {
		return nil, newNodeError(cmd, serverNodes[0], err)
	}

	// Bring up the rest of the nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range append(serverNodes[1:], agentNodes...) {
		cmd := fmt.Sprintf(`%s %s vagrant up %s &>> vagrant.log`, nodeEnvs, testOptions, node.String())
		fmt.Println(cmd)
		errg.Go(func() error {
			if _, err := RunCommand(cmd); err != nil {
				return newNodeError(cmd, node, err)
			}
			return nil
		})
		// We must wait a bit between provisioning nodes to avoid too many learners attempting to join the cluster
		if strings.Contains(node.String(), "agent") {
			time.Sleep(5 * time.Second)
		} else {
			time.Sleep(30 * time.Second)
		}
	}
	if err := errg.Wait(); err != nil {
		return nil, err
	}

	// For startup test, we don't start the cluster, so check first before
	// generating the kubeconfig file
	var kubeConfigFile string
	res, err := serverNodes[0].RunCmdOnNode("systemctl is-active k3s")
	if err != nil {
		return nil, err
	}
	if !strings.Contains(res, "inactive") && strings.Contains(res, "active") {
		kubeConfigFile, err = GenKubeconfigFile(serverNodes[0].String())
		if err != nil {
			return nil, err
		}
	}

	tc := &TestConfig{
		KubeconfigFile: kubeConfigFile,
		Servers:        serverNodes,
		Agents:         agentNodes,
	}

	return tc, nil
}

func scpK3sBinary(nodeNames []VagrantNode) error {
	for _, node := range nodeNames {
		cmd := fmt.Sprintf(`vagrant scp ../../../dist/artifacts/k3s  %s:/tmp/`, node.String())
		if _, err := RunCommand(cmd); err != nil {
			return fmt.Errorf("failed to scp k3s binary to %s: %v", node, err)
		}
		cmd = "vagrant ssh " + node.String() + " -c \"sudo mv /tmp/k3s /usr/local/bin/\""
		if _, err := RunCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

// CreateLocalCluster creates a cluster using the locally built k3s binary. The vagrant-scp plugin must be installed for
// this function to work. The binary is deployed as an airgapped install of k3s on the VMs.
func CreateLocalCluster(nodeOS string, serverCount, agentCount int) (*TestConfig, error) {

	serverNodes, agentNodes, nodeEnvs := genNodeEnvs(nodeOS, serverCount, agentCount)

	var testOptions string
	var cmd string

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}
	testOptions += " E2E_RELEASE_VERSION=skip"

	// Provision the first server node. In GitHub Actions, this also imports the VM image into libvirt, which
	// takes time and can cause the next vagrant up to fail if it is not given enough time to complete.
	cmd = fmt.Sprintf(`%s %s vagrant up --no-tty --no-provision %s &> vagrant.log`, nodeEnvs, testOptions, serverNodes[0])
	fmt.Println(cmd)
	if _, err := RunCommand(cmd); err != nil {
		return nil, newNodeError(cmd, serverNodes[0], err)
	}

	// Bring up the rest of the nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range append(serverNodes[1:], agentNodes...) {
		cmd := fmt.Sprintf(`%s %s vagrant up --no-provision %s &>> vagrant.log`, nodeEnvs, testOptions, node)
		errg.Go(func() error {
			if _, err := RunCommand(cmd); err != nil {
				return newNodeError(cmd, node, err)
			}
			return nil
		})
		// libVirt/Virtualbox needs some time between provisioning nodes
		time.Sleep(10 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return nil, err
	}

	if err := scpK3sBinary(append(serverNodes, agentNodes...)); err != nil {
		return nil, err
	}
	// Install K3s on all nodes in parallel
	errg, _ = errgroup.WithContext(context.Background())
	for _, node := range append(serverNodes, agentNodes...) {
		cmd = fmt.Sprintf(`%s %s vagrant provision %s &>> vagrant.log`, nodeEnvs, testOptions, node)
		errg.Go(func() error {
			if _, err := RunCommand(cmd); err != nil {
				return newNodeError(cmd, node, err)
			}
			return nil
		})
		// K3s needs some time between joining nodes to avoid learner issues
		time.Sleep(20 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return nil, err
	}

	// For startup test, we don't start the cluster, so check first before generating the kubeconfig file.
	// Systemctl returns a exit code of 3 when the service is inactive, so we don't check for errors
	// on the command itself.
	var kubeConfigFile string
	var err error
	res, _ := serverNodes[0].RunCmdOnNode("systemctl is-active k3s")
	if !strings.Contains(res, "inactive") && strings.Contains(res, "active") {
		kubeConfigFile, err = GenKubeconfigFile(serverNodes[0].String())
		if err != nil {
			return nil, err
		}
	}

	tc := &TestConfig{
		KubeconfigFile: kubeConfigFile,
		Servers:        serverNodes,
		Agents:         agentNodes,
	}

	return tc, nil
}

func (tc TestConfig) DeployWorkload(workload string) (string, error) {
	resourceDir := "../amd64_resource_files"
	if tc.Hardened {
		resourceDir = "../cis_amd64_resource_files"
	}
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}
	fmt.Println("\nDeploying", workload)
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl apply -f " + filename + " --kubeconfig=" + tc.KubeconfigFile
			return RunCommand(cmd)
		}
	}
	return "", nil
}

func DestroyCluster() error {
	if _, err := RunCommand("vagrant destroy -f"); err != nil {
		return err
	}
	return os.Remove("vagrant.log")
}

func FetchClusterIP(kubeconfig string, servicename string, dualStack bool) (string, error) {
	if dualStack {
		cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIPs}' --kubeconfig=" + kubeconfig
		res, err := RunCommand(cmd)
		if err != nil {
			return res, err
		}
		res = strings.ReplaceAll(res, "\"", "")
		return strings.Trim(res, "[]"), nil
	}
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	return RunCommand(cmd)
}

// FetchExternalIPs fetches the external IPs of a service
func FetchExternalIPs(kubeconfig string, servicename string) ([]string, error) {
	var externalIPs []string
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.status.loadBalancer.ingress}' --kubeconfig=" + kubeconfig
	output, err := RunCommand(cmd)
	if err != nil {
		return externalIPs, err
	}

	var svcExternalIPs []SvcExternalIP
	err = json.Unmarshal([]byte(output), &svcExternalIPs)
	if err != nil {
		return externalIPs, fmt.Errorf("Error unmarshalling JSON: %v", err)
	}

	// Iterate over externalIPs and append each IP to the ips slice
	for _, ipEntry := range svcExternalIPs {
		externalIPs = append(externalIPs, ipEntry.IP)
	}

	return externalIPs, nil
}

func FetchIngressIP(kubeconfig string) ([]string, error) {
	cmd := "kubectl get ing  ingress  -o jsonpath='{.status.loadBalancer.ingress[*].ip}' --kubeconfig=" + kubeconfig
	res, err := RunCommand(cmd)
	if err != nil {
		return nil, err
	}
	res = strings.TrimSpace(res)
	if res == "" {
		return nil, errors.New("no ingress IPs found")
	}
	return strings.Split(res, " "), nil
}

func (v VagrantNode) FetchNodeExternalIP() (string, error) {
	cmd := "ip -f inet addr show eth1| awk '/inet / {print $2}'|cut -d/ -f1"
	ipaddr, err := v.RunCmdOnNode(cmd)
	if err != nil {
		return "", err
	}
	ips := strings.Trim(ipaddr, "")
	ip := strings.Split(ips, "inet")
	nodeip := strings.TrimSpace(ip[1])
	return nodeip, nil
}

// GenKubeconfigFile extracts the kubeconfig from the given node and modifies it for use outside the VM.
func GenKubeconfigFile(nodeName string) (string, error) {
	kubeconfigFile := fmt.Sprintf("kubeconfig-%s", nodeName)
	cmd := fmt.Sprintf("vagrant scp %s:/etc/rancher/k3s/k3s.yaml ./%s", nodeName, kubeconfigFile)
	_, err := RunCommand(cmd)
	if err != nil {
		return "", err
	}

	kubeConfig, err := os.ReadFile(kubeconfigFile)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`(?m)==> vagrant:.*\n`)
	modifiedKubeConfig := re.ReplaceAllString(string(kubeConfig), "")
	vNode := VagrantNode(nodeName)
	nodeIP, err := vNode.FetchNodeExternalIP()
	if err != nil {
		return "", err
	}
	modifiedKubeConfig = strings.Replace(modifiedKubeConfig, "127.0.0.1", nodeIP, 1)
	if err := os.WriteFile(kubeconfigFile, []byte(modifiedKubeConfig), 0644); err != nil {
		return "", err
	}

	if err := os.Setenv("E2E_KUBECONFIG", kubeconfigFile); err != nil {
		return "", err
	}
	return kubeconfigFile, nil
}

func GenReport(specReport ginkgo.SpecReport) {
	state := struct {
		State string        `json:"state"`
		Name  string        `json:"name"`
		Type  string        `json:"type"`
		Time  time.Duration `json:"time"`
	}{
		State: specReport.State.String(),
		Name:  specReport.LeafNodeText,
		Type:  "k3s test",
		Time:  specReport.RunTime,
	}
	status, _ := json.Marshal(state)
	fmt.Printf("%s", status)
}

func (v VagrantNode) GetJournalLogs() (string, error) {
	cmd := "journalctl -u k3s* --no-pager"
	return v.RunCmdOnNode(cmd)
}

func TailJournalLogs(lines int, nodes []VagrantNode) string {
	logs := &strings.Builder{}
	for _, node := range nodes {
		cmd := fmt.Sprintf("journalctl -u k3s* --no-pager --lines=%d", lines)
		if l, err := node.RunCmdOnNode(cmd); err != nil {
			fmt.Fprintf(logs, "** failed to read journald log for node %s ***\n%v\n", node, err)
		} else {
			fmt.Fprintf(logs, "** journald log for node %s ***\n%s\n", node, l)
		}
	}
	return logs.String()
}

// SaveJournalLogs saves the journal logs of each node to a <NAME>-jlog.txt file.
// When used in GHA CI, the logs are uploaded as an artifact on failure.
func SaveJournalLogs(nodes []VagrantNode) error {
	for _, node := range nodes {
		lf, err := os.Create(node.String() + "-jlog.txt")
		if err != nil {
			return err
		}
		defer lf.Close()
		logs, err := node.GetJournalLogs()
		if err != nil {
			return err
		}
		if _, err := lf.Write([]byte(logs)); err != nil {
			return fmt.Errorf("failed to write %s node logs: %v", node, err)
		}
	}
	return nil
}

func GetConfig(nodes []VagrantNode) string {
	config := &strings.Builder{}
	for _, node := range nodes {
		cmd := "tar -Pc /etc/rancher/k3s/ | tar -vxPO"
		if c, err := node.RunCmdOnNode(cmd); err != nil {
			fmt.Fprintf(config, "** failed to get config for node %s ***\n%v\n", node, err)
		} else {
			fmt.Fprintf(config, "** config for node %s ***\n%s\n", node, c)
		}
	}
	return config.String()
}

// GetVagrantLog returns the logs of on vagrant commands that initialize the nodes and provision K3s on each node.
// It also attempts to fetch the systemctl logs of K3s on nodes where the k3s.service failed.
func GetVagrantLog(cErr error) string {
	var nodeErr *NodeError
	nodeJournal := ""
	if errors.As(cErr, &nodeErr) {
		nodeJournal, _ = nodeErr.Node.GetJournalLogs()
		nodeJournal = "\nNode Journal Logs:\n" + nodeJournal
	}

	log, err := os.Open("vagrant.log")
	if err != nil {
		return err.Error()
	}
	bytes, err := io.ReadAll(log)
	if err != nil {
		return err.Error()
	}
	return string(bytes) + nodeJournal
}

func ParseNodes(kubeConfig string, print bool) ([]Node, error) {
	nodes := make([]Node, 0, 10)
	nodeList := ""

	cmd := "kubectl get nodes --no-headers -o wide -A --kubeconfig=" + kubeConfig
	res, err := RunCommand(cmd)

	if err != nil {
		return nil, fmt.Errorf("unable to get nodes: %s: %v", res, err)
	}
	nodeList = strings.TrimSpace(res)
	split := strings.Split(nodeList, "\n")
	for _, rec := range split {
		if strings.TrimSpace(rec) != "" {
			fields := strings.Fields(rec)
			node := Node{
				Name:       fields[0],
				Status:     fields[1],
				Roles:      fields[2],
				InternalIP: fields[5],
			}
			if len(fields) > 6 {
				node.ExternalIP = fields[6]
			}
			nodes = append(nodes, node)
		}
	}
	if print {
		fmt.Println(nodeList)
	}
	return nodes, nil
}

func DumpPods(kubeConfig string) {
	cmd := "kubectl get pods -o wide --no-headers -A"
	res, _ := RunCommand(cmd)
	fmt.Println(strings.TrimSpace(res))
}

// RestartCluster restarts the k3s service on each node given
func RestartCluster(nodes []VagrantNode) error {
	for _, node := range nodes {
		cmd := "systemctl restart k3s* --all"
		if _, err := node.RunCmdOnNode(cmd); err != nil {
			return err
		}
	}
	return nil
}

// StartCluster starts the k3s service on each node given
func StartCluster(nodes []VagrantNode) error {
	for _, node := range nodes {
		cmd := "systemctl start k3s"
		if strings.Contains(node.String(), "agent") {
			cmd += "-agent"
		}
		if _, err := node.RunCmdOnNode(cmd); err != nil {
			return err
		}
	}
	return nil
}

// StopCluster starts the k3s service on each node given
func StopCluster(nodes []VagrantNode) error {
	for _, node := range nodes {
		cmd := "systemctl stop k3s*"
		if _, err := node.RunCmdOnNode(cmd); err != nil {
			return err
		}
	}
	return nil
}

// RunCmdOnNode executes a command from within the given node as sudo
func (v VagrantNode) RunCmdOnNode(cmd string) (string, error) {
	injectEnv := ""
	if _, ok := os.LookupEnv("E2E_GOCOVER"); ok && strings.HasPrefix(cmd, "k3s") {
		injectEnv = "GOCOVERDIR=/tmp/k3scov "
	}
	runcmd := "vagrant ssh --no-tty " + v.String() + " -c \"sudo " + injectEnv + cmd + "\""
	out, err := RunCommand(runcmd)
	// On GHA CI we see warnings about "[fog][WARNING] Unrecognized arguments: libvirt_ip_command"
	// these are added to the command output and need to be removed
	out = strings.ReplaceAll(out, "[fog][WARNING] Unrecognized arguments: libvirt_ip_command\n", "")
	if err != nil {
		return out, fmt.Errorf("failed to run command: %s on node %s: %s, %v", cmd, v.String(), out, err)
	}
	return out, nil
}

func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	if kc, ok := os.LookupEnv("E2E_KUBECONFIG"); ok {
		c.Env = append(os.Environ(), "KUBECONFIG="+kc)
	}
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("failed to run command: %s, %v", cmd, err)
	}
	return string(out), err
}

func UpgradeCluster(nodes []VagrantNode, local bool) error {
	upgradeVersion := "E2E_RELEASE_CHANNEL=commit"
	if local {
		if err := scpK3sBinary(nodes); err != nil {
			return err
		}
		upgradeVersion = "E2E_RELEASE_VERSION=skip"
	}
	for _, node := range nodes {
		cmd := upgradeVersion + " vagrant provision " + node.String()
		if out, err := RunCommand(cmd); err != nil {
			fmt.Println("Error Upgrading Cluster", out)
			return err
		}
	}
	return nil
}

func GetCoverageReport(nodes []VagrantNode) error {
	if os.Getenv("E2E_GOCOVER") == "" {
		return nil
	}
	covDirs := []string{}
	for _, node := range nodes {
		covDir := node.String() + "-cov"
		covDirs = append(covDirs, covDir)
		os.MkdirAll(covDir, 0755)
		cmd := "vagrant scp " + node.String() + ":/tmp/k3scov/* " + covDir
		if _, err := RunCommand(cmd); err != nil {
			return err
		}
	}
	coverageFile := "coverage.out"
	cmd := "go tool covdata textfmt -i=" + strings.Join(covDirs, ",") + " -o=" + coverageFile
	if out, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to generate coverage report: %s, %v", out, err)
	}

	f, err := os.ReadFile(coverageFile)
	if err != nil {
		return err
	}
	nf := strings.Replace(string(f),
		"/go/src/github.com/k3s-io/k3s/cmd/server/main.go",
		"github.com/k3s-io/k3s/cmd/server/main.go", -1)

	if err = os.WriteFile(coverageFile, []byte(nf), os.ModePerm); err != nil {
		return err
	}

	for _, covDir := range covDirs {
		if err := os.RemoveAll(covDir); err != nil {
			return err
		}
	}
	return nil
}

// GetDaemonsetReady returns the number of ready pods for the given daemonset
func GetDaemonsetReady(daemonset string, kubeConfigFile string) (int, error) {
	cmd := "kubectl get ds " + daemonset + " -o jsonpath='{range .items[*]}{.status.numberReady}' --kubeconfig=" + kubeConfigFile
	out, err := RunCommand(cmd)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

// GetPodIPs returns the IPs of all pods
func GetPodIPs(kubeConfigFile string) ([]ObjIP, error) {
	cmd := `kubectl get pods -A -o=jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.podIPs[*].ip}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return GetObjIPs(cmd)
}

// GetNodeIPs returns the IPs of all nodes
func GetNodeIPs(kubeConfigFile string) ([]ObjIP, error) {
	cmd := `kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.addresses[?(@.type == "InternalIP")].address}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return GetObjIPs(cmd)
}

// GetObjIPs executes a command to collect IPs
func GetObjIPs(cmd string) ([]ObjIP, error) {
	var objIPs []ObjIP
	res, err := RunCommand(cmd)
	if err != nil {
		return nil, err
	}
	objs := strings.Split(res, "\n")
	objs = objs[:len(objs)-1]

	for _, obj := range objs {
		fields := strings.Fields(obj)
		if len(fields) > 2 {
			objIPs = append(objIPs, ObjIP{Name: fields[0], IPv4: fields[1], IPv6: fields[2]})
		} else if len(fields) > 1 {
			if strings.Contains(fields[1], ".") {
				objIPs = append(objIPs, ObjIP{Name: fields[0], IPv4: fields[1]})
			} else {
				objIPs = append(objIPs, ObjIP{Name: fields[0], IPv6: fields[1]})
			}
		} else {
			objIPs = append(objIPs, ObjIP{Name: fields[0]})
		}
	}

	return objIPs, nil
}
