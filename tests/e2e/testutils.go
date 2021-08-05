package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
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

var config *ssh.ClientConfig
var SSHKEY string
var SSHUSER string
var err error

func checkError(e error) {
	if e != nil {
		log.Fatal(err)
		panic(e)
	}
}

func publicKey(path string) ssh.AuthMethod {
	key, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		panic(err)
	}
	return ssh.PublicKeys(signer)
}

func ConfigureSSH(host string, SSHUser string, SSHKey string) *ssh.Client {
	config = &ssh.ClientConfig{
		User: SSHUser,
		Auth: []ssh.AuthMethod{
			publicKey(SSHKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err := ssh.Dial("tcp", host, config)
	checkError(err)
	return conn
}

func runsshCommand(cmd string, conn *ssh.Client) string {
	session, err := conn.NewSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	if err := session.Run(cmd); err != nil {
		log.Println(session.Stdout)
		log.Fatal("Error on command execution", err.Error())
	}
	return fmt.Sprintf("%s", stdoutBuf.String())
}

//Runs command passed from within the node ServerIP
func RunCmdOnNode(cmd string, ServerIP string, SSHUser string, SSHKey string) string {
	Server := ServerIP + ":22"
	conn := ConfigureSSH(Server, SSHUser, SSHKey)
	res := runsshCommand(cmd, conn)
	res = strings.TrimSpace(res)
	return res
}

// RunCommand Runs command on the cluster accessing the cluster through kubeconfig file
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	time.Sleep(10 * time.Second)
	var out bytes.Buffer
	c.Stdout = &out
	err := c.Run()
	if err != nil {
		return "", errors.New(fmt.Sprintf("%s", err))
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

func DeployWorkload(workload string, kubeconfig string) (string, error) {
	cmd := "kubectl apply -f " + workload + " --kubeconfig=" + kubeconfig
	return RunCommand(cmd)
}

func FetchClusterIP(kubeconfig string, servicename string) string {
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	res, _ := RunCommand(cmd)
	return res
}

func FetchNodeExternalIP(kubeconfig string) []string {
	cmd := "kubectl get node --output=jsonpath='{range .items[*]} { .status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig=" + kubeconfig
	time.Sleep(10 * time.Second)
	res, _ := RunCommand(cmd)
	nodeExternalIP := strings.Trim(res, " ")
	nodeExternalIPs := strings.Split(nodeExternalIP, " ")
	return nodeExternalIPs
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
	var node Node
	timeElapsed := 0
	nodeList := ""
	time.Sleep(60 * time.Second)
	for timeElapsed < 420 {
		notReady := false
		cmd := "kubectl get nodes --no-headers -o wide -A --kubeconfig=" + kubeconfig
		res, _ := RunCommand(cmd)
		res = strings.TrimSpace(res)
		nodeList = res
		split := strings.Split(res, "\n")
		for _, rec := range split {
			fields := strings.Fields(string(rec))
			node.Name = fields[0]
			node.Status = fields[1]
			node.Roles = fields[2]
			node.InternalIP = fields[5]
			node.ExternalIP = fields[6]
			nodes = append(nodes, node)
			if node.Status != "Ready" {
				notReady = true
				break
			}
		}
		if notReady == false {
			break
		}
		time.Sleep(5 * time.Second)
		timeElapsed = timeElapsed + 10
	}
	if printres {
		fmt.Println(nodeList)
	}
	return nodes
}

func ParsePod(kubeconfig string, printres bool) []Pod {
	pods := make([]Pod, 0, 10)
	var pod Pod
	timeElapsed := 0
	time.Sleep(60 * time.Second)
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
			pod.NameSpace = fields[0]
			pod.Name = fields[1]
			pod.Ready = fields[2]
			pod.Status = fields[3]
			pod.Restarts = fields[4]
			pod.NodeIP = fields[6]
			pod.Node = fields[7]
			pods = append(pods, pod)
			if strings.HasPrefix(pod.Name, "helm-install") && pod.Status != "Completed" {
				helmPodsNR = true
				break
			} else if !strings.HasPrefix(pod.Name, "helm-install") && pod.Status != "Running" {

				systemPodsNR = true
				break
			}
			time.Sleep(10 * time.Second)
			timeElapsed = timeElapsed + 10
		}
		if systemPodsNR == false && helmPodsNR == false {
			break
		}
	}
	if printres {
		fmt.Println(podList)
	}
	return pods
}
