package rootless

import (
	"fmt"
	"os"
	"strings"

	"github.com/k3s-io/k3s/tests/e2e"
)

// RunCmdOne2e.Node executes a command from within the given node as sudo
func RunCmdOnRootlesNode(cmd string, nodename string) (string, error) {
	injectEnv := ""
	if _, ok := os.LookupEnv("E2E_GOCOVER"); ok && strings.HasPrefix(cmd, "k3s") {
		injectEnv = "GOCOVERDIR=/tmp/k3scov "
	}
	runcmd := "sudo vagrant ssh " + nodename + " -c \"" + injectEnv + cmd + "\""
	out, err := e2e.RunCommand(runcmd)
	if err != nil {
		return out, fmt.Errorf("failed to run command: %s on node %s: %s, %v", cmd, nodename, out, err)
	}
	return out, nil
}

func formatPods(input string) ([]e2e.Pod, error) {
	pods := make([]e2e.Pod, 0, 10)
	input = strings.TrimSpace(input)
	split := strings.Split(input, "\n")
	for _, rec := range split {
		fields := strings.Fields(string(rec))
		if len(fields) < 8 {
			return nil, fmt.Errorf("invalid pod record: %s", rec)
		}
		pod := e2e.Pod{
			NameSpace: fields[0],
			Name:      fields[1],
			Ready:     fields[2],
			Status:    fields[3],
			Restarts:  fields[4],
			IP:        fields[6],
			Node:      fields[7],
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

func ParsePods(print bool, node string) ([]e2e.Pod, error) {
	podList := ""

	cmd := "kubectl get pods -o wide --no-headers -A "
	res, _ := RunCmdOnRootlesNode(cmd, node)
	podList = strings.TrimSpace(res)

	pods, err := formatPods(res)
	if err != nil {
		return nil, err
	}
	if print {
		fmt.Println(podList)
	}
	return pods, nil
}

func ParseNodes(print bool, node string) ([]e2e.Node, error) {
	nodes := make([]e2e.Node, 0, 10)
	nodeList := ""

	cmd := "kubectl get nodes --no-headers -o wide -A"
	res, err := RunCmdOnRootlesNode(cmd, node)

	if err != nil {
		return nil, fmt.Errorf("unable to get nodes: %s: %v", res, err)
	}
	if res == "No resources found\n" {
		return nil, fmt.Errorf("unable to get nodes: %s: %v", res, nil)
	}
	nodeList = strings.TrimSpace(res)
	split := strings.Split(nodeList, "\n")
	for _, rec := range split {
		if strings.TrimSpace(rec) != "" {
			fields := strings.Fields(rec)
			node := e2e.Node{
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
