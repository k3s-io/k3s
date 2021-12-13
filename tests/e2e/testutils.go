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
	c.Stdout = &out
	if err := c.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// RunCommand Runs command on the cluster accessing the cluster through kubeconfig file
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	time.Sleep(10 * time.Second)
	var out bytes.Buffer
	c.Stdout = &out

	if err := c.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

//Used to count the pods using prefix passed in the list of pods
func CountOfStringInSlice(str string, pods []Pod) int {
	count := 0
	for _, pod := range pods {
		if strings.Contains(pod.Name, str) {
			count++
		}
	}
	return count
}

func CreateCluster(nodeos string, serverCount int, agentCount int) ([]string, []string, error) {
	scount := make([]int, serverCount)
	serverNodenames := make([]string, serverCount+1)
	for i := range scount {
		serverNodenames[i] = "server-" + strconv.Itoa(i)
	}

	acount := make([]int, agentCount)
	agentNodenames := make([]string, agentCount+1)
	for i := range acount {
		agentNodenames[i] = "agent-" + strconv.Itoa(i)
	}

	cmd := "vagrant up"
	if _, err := RunCommand(cmd); err != nil {
		fmt.Printf("Error Creating Cluster")
		return nil, nil, err
	}

	return serverNodenames, agentNodenames, nil
}

func DestroyCluster() error {
	_, err := RunCommand("vagrant destroy -f")
	return err
}

func GenKubeConfigFile(serverName string) (string, error) {
	cmd := fmt.Sprintf("vagrant ssh %s -c \"cat /etc/rancher/k3s/k3s.yaml\"", serverName)
	kubeConfig, err := RunCommand(cmd)
	if err != nil {
		return "", err
	}
	nodeIp := FetchNodeExternalIP(serverName)
	kubeConfig = strings.Replace(kubeConfig, "127.0.0.1", nodeIp, 1)
	fmt.Println("KubeConfig\n", kubeConfig)
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

func FetchClusterIP(kubeconfig string, servicename string) string {
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	res, _ := RunCommand(cmd)
	return res
}

func FetchNodeExternalIP(nodename string) string {
	cmd := "vagrant ssh " + nodename + " -c  \"ip -f inet addr show eth1| awk '/inet / {print $2}'|cut -d/ -f1\""
	ipaddr, _ := RunCommand(cmd)
	ips := strings.Trim(ipaddr, "")
	ip := strings.Split(ips, "inet")
	nodeip := strings.TrimSpace(ip[1])
	return nodeip
}
func FetchIngressIP(kubeconfig string) []string {
	cmd := "kubectl get ing  ingress  -o jsonpath='{.status.loadBalancer.ingress[*].ip}' --kubeconfig=" + kubeconfig
	time.Sleep(10 * time.Second)
	res, _ := RunCommand(cmd)

	ingressIp := strings.Trim(res, " ")
	ingressIps := strings.Split(ingressIp, " ")
	return ingressIps
}

func ParseNode(kubeconfig string, printres bool) []Node {
	nodes := make([]Node, 0, 10)
	timeElapsed := 0
	nodeList := ""

	for timeElapsed < 420 {
		ready := false
		cmd := "kubectl get nodes --no-headers -o wide -A --kubeconfig=" + kubeconfig
		res, _ := RunCommand(cmd)
		nodeList = strings.TrimSpace(res)
		split := strings.Split(res, "\n")
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
					ready = true
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
	if printres {
		fmt.Println(nodeList)
	}
	return nodes
}

func ParsePod(kubeconfig string, printres bool) []Pod {
	pods := make([]Pod, 0, 10)
	timeElapsed := 0
	podList := ""

	for timeElapsed < 420 {
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
	if printres {
		fmt.Println(podList)
	}
	return pods
}
