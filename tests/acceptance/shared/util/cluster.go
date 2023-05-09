package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/gomega"
)

type Node struct {
	Name       string
	Status     string
	Roles      string
	Version    string
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

// ParseNodes parses the nodes from the kubectl get nodes command
// and returns a list of nodes
func ParseNodes(print bool) ([]Node, error) {
	nodes := make([]Node, 0, 10)
	res, err := RunCommandHost(GetNodesWide + KubeConfigFile)
	if err != nil {
		return nil, err
	}

	nodeList := strings.TrimSpace(res)
	split := strings.Split(nodeList, "\n")
	for _, rec := range split {
		if strings.TrimSpace(rec) != "" {
			fields := strings.Fields(rec)
			node := Node{
				Name:       fields[0],
				Status:     fields[1],
				Roles:      fields[2],
				Version:    fields[4],
				InternalIP: fields[5],
				ExternalIP: fields[6],
			}
			nodes = append(nodes, node)
		}
	}
	if print {
		fmt.Println(nodeList)
	}

	return nodes, nil
}

// ParsePods parses the pods from the kubectl get pods command
// and returns a list of pods
func ParsePods(print bool) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	podList := ""

	res, _ := RunCommandHost(GetPodsWide + KubeConfigFile)
	res = strings.TrimSpace(res)
	podList = res

	split := strings.Split(res, "\n")
	for _, rec := range split {
		fields := strings.Fields(rec)
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

// ManageWorkload creates or deletes a workload  | action: create or delete
func ManageWorkload(action, workload, arch string) (string, error) {
	if action != "create" && action != "delete" {
		return "", fmt.Errorf("invalid action: %s. Must be 'create' or 'delete'", action)
	}

	resourceDir := GetBasepath() + "/shared/amd64workloads/"
	if arch == "arm64" {
		resourceDir = GetBasepath() + "/shared/armworkloads/"
	}

	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}

	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			var cmd string
			if action == "create" {
				fmt.Println("\nDeploying", workload)
				cmd = "kubectl apply -f " + filename + " --kubeconfig=" + KubeConfigFile
			} else {
				fmt.Println("\nRemoving", workload)
				cmd = "kubectl delete -f " + filename + " --kubeconfig=" + KubeConfigFile
			}
			res, err := RunCommandHost(cmd)

			if action == "delete" {
				gomega.Eventually(func(g gomega.Gomega) {
					isDeleted, err := IsWorkloadDeleted(workload)
					g.Expect(err).To(gomega.BeNil())
					g.Expect(isDeleted).To(gomega.BeTrue(),
						"Workload should be deleted")
				}, "60s", "5s").Should(gomega.Succeed())
			}
			return res, err
		}
	}

	return "", nil
}

// IsWorkloadDeleted checks if the workload is deleted
func IsWorkloadDeleted(workload string) (bool, error) {
	res, err := RunCommandHost(GetAll + KubeConfigFile)
	if err != nil {
		return false, err
	}

	return !strings.Contains(res, workload), nil
}

// FetchClusterIP returns the cluster IP of the service
func FetchClusterIP(serviceName string) (string, error) {
	cmd := "kubectl get svc " + serviceName + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" +
		KubeConfigFile
	return RunCommandHost(cmd)
}

// FetchNodeExternalIP returns the external IP of the node
func FetchNodeExternalIP() []string {
	time.Sleep(10 * time.Second)

	res, _ := RunCommandHost(GetExternalNodeIp + KubeConfigFile)
	nodeExternalIP := strings.Trim(res, " ")
	nodeExternalIPs := strings.Split(nodeExternalIP, " ")

	return nodeExternalIPs
}

// FetchIngressIP returns the ingress IP
func FetchIngressIP() ([]string, error) {
	res, err := RunCommandHost(GetIngress + KubeConfigFile)
	if err != nil {
		return nil, err
	}

	ingressIP := strings.Trim(res, " ")
	fmt.Println(ingressIP)
	ingressIPs := strings.Split(ingressIP, " ")
	return ingressIPs, nil
}

func ReadDataPod(name string) error {
	podName, err := KubectlCommand(
		"host",
		"get",
		"pods",
		"-l app="+name+" -o jsonpath={.items[0].metadata.name}",
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	cmd := "kubectl exec " + podName + " --kubeconfig=" + KubeConfigFile +
		" -- cat /data/test"
	res, err := RunCommandHost(cmd)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(res).Should(gomega.ContainSubstring(TestingLocalPath))

	return nil
}

func WriteDataPod(name string) error {
	podName, err := KubectlCommand(
		"host",
		"get",
		"pods",
		"-l app="+name+" -o jsonpath={.items[0].metadata.name}",
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	cmd := "kubectl exec " + podName + " --kubeconfig=" + KubeConfigFile +
		" -- sh -c 'echo testing local path > /data/test'"
	_, err = RunCommandHost(cmd)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return nil
}

// KubectlCommand return results from various commands, it receives an "action" , source and args.
// destination = host or node
// action = get,describe...
// source = pods, node , exec, service ...
// args   = the rest of your command arguments.
func KubectlCommand(destination, action, source string, args ...string) (string, error) {
	var cmd string
	var res string
	var err error
	kubeconfigFlag := " --kubeconfig=" + KubeConfigFile

	if destination == "host" {
		cmd = addKubectlCommand(action, source, args) + kubeconfigFlag
		res, err = RunCommandHost(cmd)
		if err != nil {
			return res, err
		}
	} else if destination == "node" {
		cmd = addKubectlCommand(action, source, args) + kubeconfigFlag
		ips := FetchNodeExternalIP()
		for _, ip := range ips {
			res, err = RunCmdOnNode(cmd, ip)
			if err != nil {
				return "", err
			}
		}
	} else {
		return "", fmt.Errorf("invalid destination: %s", destination)
	}

	return res, nil
}

func addKubectlCommand(action, source string, args []string) string {
	commandShort := map[string]string{
		"get":      "kubectl get",
		"describe": "kubectl describe",
		"exec":     "kubectl exec",
		"delete":   "kubectl delete",
		"apply":    "kubectl apply",
	}

	cmdPrefix, ok := commandShort[action]
	if !ok {
		cmdPrefix = action
	}

	return cmdPrefix + " " + source + " " + strings.Join(args, " ")
}

// FetchServiceNodePort returns the node port of the service
func FetchServiceNodePort(serviceName string) (string, error) {
	cmd := "kubectl get service " + serviceName + " --kubeconfig=" + KubeConfigFile +
		" --output jsonpath=\"{.spec.ports[0].nodePort}\""
	nodeport, err := RunCommandHost(cmd)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return nodeport, nil
}

// RestartCluster restarts the rke2 service on each node given by external IP.
func RestartCluster(ip string) error {
	if _, err := RunCmdOnNode("sudo systemctl restart k3s*", ip); err != nil {
		return K3sError{
			ErrorSource: "RestartCluster - sudo systemctl restart k3s-*",
			Message:     "something went wrong while restarting",
			Err:         err,
		}
	}
	time.Sleep(20 * time.Second)

	return nil
}

// UpgradeClusterInRunTime upgrades the cluster in runtime by running the curl command
func UpgradeClusterInRunTime(installType, value string) error {
	cmd := fmt.Sprintf(Upgradek3s, installType, value)

	nodeExternalIps := FetchNodeExternalIP()
	for _, ip := range nodeExternalIps {
		if _, err := RunCmdOnNode(cmd, ip); err != nil {
			return err
		}
	}
	cmd = "k3s --version"
	gomega.Eventually(func(g gomega.Gomega) {
		res, err := RunCommandHost("k3s --version")
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(res).Should(gomega.ContainSubstring(value))
	}, "420s", "5s").Should(gomega.Succeed())

	return nil
}
