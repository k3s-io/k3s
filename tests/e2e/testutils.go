package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	json "github.com/json-iterator/go"
	ginkgo "github.com/onsi/ginkgo/v2"
	"golang.org/x/sync/errgroup"
)

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

type NodeError struct {
	Node string
	Cmd  string
	Err  error
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

func newNodeError(cmd, node string, err error) *NodeError {
	return &NodeError{
		Cmd:  cmd,
		Node: node,
		Err:  err,
	}
}

func CountOfStringInSlice(str string, pods []Pod) int {
	count := 0
	for _, pod := range pods {
		if strings.Contains(pod.Name, str) {
			count++
		}
	}
	return count
}

// genNodeEnvs generates the node and testing environment variables for vagrant up
func genNodeEnvs(nodeOS string, serverCount, agentCount int) ([]string, []string, string) {
	serverNodeNames := make([]string, serverCount)
	for i := 0; i < serverCount; i++ {
		serverNodeNames[i] = "server-" + strconv.Itoa(i)
	}
	agentNodeNames := make([]string, agentCount)
	for i := 0; i < agentCount; i++ {
		agentNodeNames[i] = "agent-" + strconv.Itoa(i)
	}

	nodeRoles := strings.Join(serverNodeNames, " ") + " " + strings.Join(agentNodeNames, " ")
	nodeRoles = strings.TrimSpace(nodeRoles)

	nodeBoxes := strings.Repeat(nodeOS+" ", serverCount+agentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)

	nodeEnvs := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s"`, nodeRoles, nodeBoxes)

	return serverNodeNames, agentNodeNames, nodeEnvs
}

func CreateCluster(nodeOS string, serverCount, agentCount int) ([]string, []string, error) {

	serverNodeNames, agentNodeNames, nodeEnvs := genNodeEnvs(nodeOS, serverCount, agentCount)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}
	// Bring up the first server node
	cmd := fmt.Sprintf(`%s %s vagrant up %s &> vagrant.log`, nodeEnvs, testOptions, serverNodeNames[0])

	fmt.Println(cmd)
	if _, err := RunCommand(cmd); err != nil {
		return nil, nil, newNodeError(cmd, serverNodeNames[0], err)
	}
	// Bring up the rest of the nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range append(serverNodeNames[1:], agentNodeNames...) {
		cmd := fmt.Sprintf(`%s %s vagrant up %s &>> vagrant.log`, nodeEnvs, testOptions, node)
		errg.Go(func() error {
			if _, err := RunCommand(cmd); err != nil {
				return newNodeError(cmd, node, err)
			}
			return nil
		})
		// We must wait a bit between provisioning nodes to avoid too many learners attempting to join the cluster
		time.Sleep(20 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return nil, nil, err
	}

	return serverNodeNames, agentNodeNames, nil
}

// CreateLocalCluster creates a cluster using the locally built k3s binary. The vagrant-scp plugin must be installed for
// this function to work. The binary is deployed as an airgapped install of k3s on the VMs.
// This is intended only for local testing purposes when writing a new E2E test.
func CreateLocalCluster(nodeOS string, serverCount, agentCount int) ([]string, []string, error) {

	serverNodeNames, agentNodeNames, nodeEnvs := genNodeEnvs(nodeOS, serverCount, agentCount)

	var testOptions string
	var cmd string

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}
	testOptions += " E2E_RELEASE_VERSION=skip"

	// Bring up the all of the nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for i, node := range append(serverNodeNames, agentNodeNames...) {
		if i == 0 {
			cmd = fmt.Sprintf(`%s %s vagrant up --no-provision %s &> vagrant.log`, nodeEnvs, testOptions, node)
		} else {
			cmd = fmt.Sprintf(`%s %s vagrant up --no-provision %s &>> vagrant.log`, nodeEnvs, testOptions, node)
		}
		errg.Go(func() error {
			if _, err := RunCommand(cmd); err != nil {
				return fmt.Errorf("failed initializing nodes: %s: %v", cmd, err)
			}
			return nil
		})
		// libVirt/Virtualbox needs some time between provisioning nodes
		time.Sleep(10 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return nil, nil, err
	}
	for _, node := range append(serverNodeNames, agentNodeNames...) {
		cmd = fmt.Sprintf(`vagrant scp ../../../dist/artifacts/k3s  %s:/tmp/`, node)
		if _, err := RunCommand(cmd); err != nil {
			return nil, nil, fmt.Errorf("failed to scp k3s binary to %s: %v", node, err)
		}
		if _, err := RunCmdOnNode("sudo mv /tmp/k3s /usr/local/bin/", node); err != nil {
			return nil, nil, err
		}
	}

	// Install K3s on all nodes in parallel
	errg, _ = errgroup.WithContext(context.Background())
	for _, node := range append(serverNodeNames, agentNodeNames...) {
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
		return nil, nil, err
	}

	return serverNodeNames, agentNodeNames, nil
}

func DeployWorkload(workload, kubeconfig string, hardened bool) (string, error) {
	resourceDir := "../amd64_resource_files"
	if hardened {
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
			cmd := "kubectl apply -f " + filename + " --kubeconfig=" + kubeconfig
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

func FetchIngressIP(kubeconfig string) ([]string, error) {
	cmd := "kubectl get ing  ingress  -o jsonpath='{.status.loadBalancer.ingress[*].ip}' --kubeconfig=" + kubeconfig
	res, err := RunCommand(cmd)
	if err != nil {
		return nil, err
	}
	ingressIP := strings.Trim(res, " ")
	ingressIPs := strings.Split(ingressIP, " ")
	return ingressIPs, nil
}

func FetchNodeExternalIP(nodename string) (string, error) {
	cmd := "vagrant ssh " + nodename + " -c  \"ip -f inet addr show eth1| awk '/inet / {print $2}'|cut -d/ -f1\""
	ipaddr, err := RunCommand(cmd)
	if err != nil {
		return "", err
	}
	ips := strings.Trim(ipaddr, "")
	ip := strings.Split(ips, "inet")
	nodeip := strings.TrimSpace(ip[1])
	return nodeip, nil
}

func GenKubeConfigFile(serverName string) (string, error) {
	cmd := fmt.Sprintf("vagrant ssh %s -c \"sudo cat /etc/rancher/k3s/k3s.yaml\"", serverName)
	kubeConfig, err := RunCommand(cmd)
	if err != nil {
		return "", err
	}
	nodeIP, err := FetchNodeExternalIP(serverName)
	if err != nil {
		return "", err
	}
	kubeConfig = strings.Replace(kubeConfig, "127.0.0.1", nodeIP, 1)
	kubeConfigFile := fmt.Sprintf("kubeconfig-%s", serverName)
	if err := os.WriteFile(kubeConfigFile, []byte(kubeConfig), 0644); err != nil {
		return "", err
	}
	return kubeConfigFile, nil
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

// GetVagrantLog returns the logs of on vagrant commands that initialize the nodes and provision K3s on each node.
// It also attempts to fetch the systemctl logs of K3s on nodes where the k3s.service failed.
func GetVagrantLog(cErr error) string {
	var nodeErr *NodeError
	nodeJournal := ""
	if errors.As(cErr, &nodeErr) {
		nodeJournal, _ = RunCommand("vagrant ssh " + nodeErr.Node + " -c \"sudo journalctl -u k3s* --no-pager\"")
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
		return nil, err
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

func ParsePods(kubeConfig string, print bool) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	podList := ""

	cmd := "kubectl get pods -o wide --no-headers -A --kubeconfig=" + kubeConfig
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
	if print {
		fmt.Println(podList)
	}
	return pods, nil
}

// RestartCluster restarts the k3s service on each node given
func RestartCluster(nodeNames []string) error {
	for _, nodeName := range nodeNames {
		cmd := "sudo systemctl restart k3s"
		if _, err := RunCmdOnNode(cmd, nodeName); err != nil {
			return err
		}
	}
	return nil
}

// RunCmdOnNode executes a command from within the given node
func RunCmdOnNode(cmd string, nodename string) (string, error) {
	runcmd := "vagrant ssh -c \"" + cmd + "\" " + nodename
	out, err := RunCommand(runcmd)
	if err != nil {
		return out, fmt.Errorf("failed to run command %s on node %s: %v", cmd, nodename, err)
	}
	return out, nil
}

// RunCommand executes a command on the host
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	out, err := c.CombinedOutput()
	return string(out), err
}

func UpgradeCluster(serverNodeNames []string, agentNodeNames []string) error {
	for _, nodeName := range serverNodeNames {
		cmd := "E2E_RELEASE_CHANNEL=commit vagrant provision " + nodeName
		fmt.Println(cmd)
		if out, err := RunCommand(cmd); err != nil {
			fmt.Println("Error Upgrading Cluster", out)
			return err
		}
	}
	for _, nodeName := range agentNodeNames {
		cmd := "E2E_RELEASE_CHANNEL=commit vagrant provision " + nodeName
		if _, err := RunCommand(cmd); err != nil {
			fmt.Println("Error Upgrading Cluster", err)
			return err
		}
	}
	return nil
}

// getPodIPs returns the IPs of all pods
func GetPodIPs(kubeConfigFile string) ([]ObjIP, error) {
	cmd := `kubectl get pods -A -o=jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.podIPs[*].ip}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return GetObjIPs(cmd)
}

// getNodeIPs returns the IPs of all nodes
func GetNodeIPs(kubeConfigFile string) ([]ObjIP, error) {
	cmd := `kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.addresses[?(@.type == "InternalIP")].address}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return GetObjIPs(cmd)
}

// getObjIPs executes a command to collect IPs
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
			objIPs = append(objIPs, ObjIP{Name: fields[0], IPv4: fields[1]})
		} else {
			objIPs = append(objIPs, ObjIP{Name: fields[0]})
		}
	}

	return objIPs, nil
}
