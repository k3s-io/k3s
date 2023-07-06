package shared

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	KubeConfigFile string
	AwsUser        string
	AccessKey      string
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
	res, err := RunCommandHost("kubectl get nodes --no-headers -o wide -A --kubeconfig=" + KubeConfigFile)
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

	res, _ := RunCommandHost("kubectl get pods -o wide --no-headers -A --kubeconfig=" + KubeConfigFile)
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

// ManageWorkload creates or deletes a workload based on the action: create or delete.
func ManageWorkload(action, workload, arch string) (string, error) {
	if action != "create" && action != "delete" {
		return "", fmt.Errorf("invalid action: %s. Must be 'create' or 'delete'", action)
	}
	var res string
	var err error

	resourceDir := GetBasepath() + "/acceptance/workloads/amd64/"
	if arch == "arm64" {
		resourceDir = GetBasepath() + "/acceptance/workloads/arm/"
	}

	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}

	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			if action == "create" {
				res, err = createWorkload(workload, filename)
				if err != nil {
					return "", fmt.Errorf("failed to create workload %s: %s", workload, err)
				}
			} else {
				err = deleteWorkload(workload, filename)
				if err != nil {
					return "", fmt.Errorf("failed to delete workload %s: %s", workload, err)
				}
			}
			return res, err
		}
	}

	return "", fmt.Errorf("workload %s not found", workload)
}

func createWorkload(workload, filename string) (string, error) {
	fmt.Println("\nDeploying", workload)
	cmd := "kubectl apply -f " + filename + " --kubeconfig=" + KubeConfigFile

	return RunCommandHost(cmd)
}

// deleteWorkload deletes a workload and asserts that the workload is deleted.
func deleteWorkload(workload, filename string) error {
	fmt.Println("\nRemoving", workload)
	cmd := "kubectl delete -f " + filename + " --kubeconfig=" + KubeConfigFile

	_, err := RunCommandHost(cmd)
	if err != nil {
		return fmt.Errorf("failed to run kubectl delete: %v", err)
	}

	timeout := time.After(60 * time.Second)
	tick := time.Tick(5 * time.Second)

	for {
		select {
		case <-timeout:
			return errors.New("workload deletion timed out")
		case <-tick:
			res, err := RunCommandHost("kubectl get all -A --kubeconfig=" + KubeConfigFile)
			if err != nil {
				return err
			}
			isDeleted := !strings.Contains(res, workload)
			if isDeleted {
				return nil
			}
		}
	}
}

// FetchClusterIP returns the cluster IP of the service
func FetchClusterIP(serviceName string) (string, error) {
	cmd := "kubectl get svc " + serviceName + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" +
		KubeConfigFile
	return RunCommandHost(cmd)
}

// FetchNodeExternalIP returns the external IP of the node
func FetchNodeExternalIP() []string {
	res, _ := RunCommandHost("kubectl get node --output=jsonpath='{range .items[*]}" +
		" {.status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig=" + KubeConfigFile)
	nodeExternalIP := strings.Trim(res, " ")
	nodeExternalIPs := strings.Split(nodeExternalIP, " ")

	return nodeExternalIPs
}

// FetchIngressIP returns the ingress IP
func FetchIngressIP() ([]string, error) {
	getIngress := "kubectl get ingress -o jsonpath='{.items[0].status.loadBalancer.ingress[*].ip}' " +
		"--kubeconfig="
	res, err := RunCommandHost(getIngress + KubeConfigFile)
	if err != nil {
		return nil, err
	}

	ingressIP := strings.Trim(res, " ")
	ingressIPs := strings.Split(ingressIP, " ")

	return ingressIPs, nil
}

// ReadDataPod reads the data from the pod
func ReadDataPod(name string) (string, error) {
	podName, err := KubectlCommand(
		"host",
		"get",
		"pods",
		"-l app="+name+" -o jsonpath={.items[0].metadata.name}",
	)
	if err != nil {
		return "", err
	}

	cmd := "kubectl exec " + podName + " --kubeconfig=" + KubeConfigFile +
		" -- cat /data/test"
	return RunCommandHost(cmd)
}

// WriteDataPod writes data to the pod
func WriteDataPod(name string) (string, error) {
	podName, err := KubectlCommand(
		"host",
		"get",
		"pods",
		"-l app="+name+" -o jsonpath={.items[0].metadata.name}",
	)
	if err != nil {
		return "", err
	}

	cmd := "kubectl exec " + podName + " --kubeconfig=" + KubeConfigFile +
		" -- sh -c 'echo testing local path > /data/test' "

	return RunCommandHost(cmd)
}

// KubectlCommand return results from various commands, it receives an "action" , source and args.
//
// destination = host or node
//
// action = get,describe...
//
// source = pods, node , exec, service ...
//
// args   = the rest of your command arguments.
func KubectlCommand(destination, action, source string, args ...string) (string, error) {
	kubeconfigFlag := " --kubeconfig=" + KubeConfigFile
	shortCmd := map[string]string{
		"get":      "kubectl get",
		"describe": "kubectl describe",
		"exec":     "kubectl exec",
		"delete":   "kubectl delete",
		"apply":    "kubectl apply",
	}

	cmdPrefix, ok := shortCmd[action]
	if !ok {
		cmdPrefix = action
	}

	cmd := cmdPrefix + " " + source + " " + strings.Join(args, " ") + kubeconfigFlag

	var res string
	var err error
	if destination == "host" {
		res, err = RunCommandHost(cmd)
		if err != nil {
			return "", err
		}
	} else if destination == "node" {
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

// FetchServiceNodePort returns the node port of the service
func FetchServiceNodePort(serviceName string) (string, error) {
	cmd := "kubectl get service " + serviceName + " --kubeconfig=" + KubeConfigFile +
		" --output jsonpath=\"{.spec.ports[0].nodePort}\""
	nodeport, err := RunCommandHost(cmd)
	if err != nil {
		return "", err
	}

	return nodeport, nil
}

// RestartCluster restarts the k3s service on each node given by external IP.
func RestartCluster(ip string) (string, error) {
	return RunCmdOnNode("sudo systemctl restart k3s*", ip)
}
