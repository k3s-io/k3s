package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
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

//Runs command passed from within the node ServerIP
func RunCmdOnNode(cmd string, nodename string) (string, error) {
	runcmd := "vagrant ssh server-0 -c " + cmd
	c := exec.Command("bash", "-c", runcmd)

	var out bytes.Buffer
	var errOut bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errOut
	if err := c.Run(); err != nil {
		return errOut.String(), err
	}
	return out.String(), nil
}

// RunCommand Runs command on the cluster accessing the cluster through kubeconfig file
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	var out bytes.Buffer
	c.Stdout = &out
	if err := c.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func CreateCluster(nodeos string, serverCount int, agentCount int) ([]string, []string, error) {
	serverNodenames := make([]string, serverCount+1)
	for i := 0; i < serverCount; i++ {
		serverNodenames[i] = "server-" + strconv.Itoa(i)
	}

	agentNodenames := make([]string, agentCount+1)
	for i := 0; i < agentCount; i++ {
		agentNodenames[i] = "agent-" + strconv.Itoa(i)
	}
	nodeRoles := strings.Join(serverNodenames, " ") + strings.Join(agentNodenames, " ")
	nodeRoles = strings.TrimSpace(nodeRoles)
	nodeBoxes := strings.Repeat(nodeos+" ", serverCount+agentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)
	cmd := fmt.Sprintf("NODE_ROLES=\"%s\" NODE_BOXES=\"%s\" vagrant up &> vagrant.log", nodeRoles, nodeBoxes)
	if out, err := RunCommand(cmd); err != nil {
		fmt.Println("Error Creating Cluster", out)
		return nil, nil, err
	}

	return serverNodenames, agentNodenames, nil
}

func DestroyCluster() error {
	if _, err := RunCommand("vagrant destroy -f"); err != nil {
		return err
	}
	return os.Remove("vagrant.log")
}

func GenKubeConfigFile(serverName string) (string, error) {
	cmd := fmt.Sprintf("vagrant ssh %s -c \"cat /etc/rancher/k3s/k3s.yaml\"", serverName)
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

func DeployWorkload(workload string, kubeconfig string) (string, error) {
	cmd := "kubectl apply -f " + workload + " --kubeconfig=" + kubeconfig
	return RunCommand(cmd)
}

func FetchClusterIP(kubeconfig string, servicename string) (string, error) {
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	return RunCommand(cmd)
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

func ParseNode(kubeConfig string, debug bool) ([]Node, error) {
	nodes := make([]Node, 0, 10)
	timeElapsed := 0
	timeMax := 240
	nodeList := ""

	for timeElapsed < timeMax {
		ready := true
		cmd := "kubectl get nodes --no-headers -o wide -A --kubeconfig=" + kubeConfig
		res, err := RunCommand(cmd)
		if err != nil {
			return nil, err
		}
		fmt.Println(res)
		nodeList = strings.TrimSpace(res)
		fmt.Println(nodeList)
		split := strings.Split(nodeList, "\n")
		fmt.Println(split)
		for _, rec := range split {
			if strings.TrimSpace(rec) != "" {
				fields := strings.Fields(string(rec))
				node := Node{
					Name:       fields[0],
					Status:     fields[1],
					Roles:      fields[2],
					InternalIP: fields[5],
					ExternalIP: fields[6],
				}
				nodes = append(nodes, node)
				if node.Status != "Ready" {
					ready = false
					break
				}
			}
		}
		if ready {
			break
		}
		time.Sleep(5 * time.Second)
		timeElapsed = timeElapsed + 5
	}
	if debug {
		fmt.Println(nodeList)
	}
	if timeElapsed >= timeMax {
		return nil, fmt.Errorf("timeout exceeded on ParseNode")
	}
	return nodes, nil
}

func ParsePod(kubeconfig string, debug bool) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	timeElapsed := 0
	podList := ""
	timeMax := 240

	for timeElapsed < timeMax {
		helmPodsNR := false
		systemPodsNR := false
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
			if strings.HasPrefix(pod.Name, "helm-install") && pod.Status != "Completed" {
				helmPodsNR = true
				break
			} else if !strings.HasPrefix(pod.Name, "helm-install") && pod.Status != "Running" {
				systemPodsNR = true
				break
			}
			time.Sleep(5 * time.Second)
			timeElapsed = timeElapsed + 5
		}
		if !systemPodsNR && !helmPodsNR {
			break
		}
	}
	if debug {
		fmt.Println(podList)
	}
	if timeElapsed >= timeMax {
		return nil, fmt.Errorf("timeout exceeded on ParsePod")
	}
	return pods, nil
}
