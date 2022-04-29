package e2e

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
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

func runsshCommand(cmd string, conn *ssh.Client) (string, error) {
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
	return fmt.Sprintf("%s", stdoutBuf.String()), err
}

// nodeOs: ubuntu centos7 centos8 sles15
// clusterType arm, etcd externaldb, if external_db var is not "" picks database from the vars file,
// resourceName: name to resource created timestamp attached

// RunCmdOnNode executes a command from within the given node
func RunCmdOnNode(cmd string, ServerIP string, SSHUser string, SSHKey string) (string, error) {
	Server := ServerIP + ":22"
	conn := ConfigureSSH(Server, SSHUser, SSHKey)
	res, err := runsshCommand(cmd, conn)
	res = strings.TrimSpace(res)
	return res, err
}

// RunCommand executes a command on the host
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	out, err := c.CombinedOutput()
	return string(out), err
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

func DeployWorkload(workload, kubeconfig string, arch string) (string, error) {
	resourceDir := "./amd64_resource_files"
	if arch == "arm64" {
		resourceDir = "./arm_resource_files"
	}
	files, err := ioutil.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}
	fmt.Println("\nDeploying", workload)
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl apply -f " + filename + " --kubeconfig=" + kubeconfig
			fmt.Println(cmd)
			return RunCommand(cmd)
		}
	}
	return "", nil
}

func FetchClusterIP(kubeconfig string, servicename string) (string, error) {
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	fmt.Println(cmd)
	return RunCommand(cmd)
}

func FetchNodeExternalIP(kubeconfig string) []string {
	cmd := "kubectl get node --output=jsonpath='{range .items[*]} { .status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig=" + kubeconfig
	time.Sleep(10 * time.Second)
	res, _ := RunCommand(cmd)
	nodeExternalIP := strings.Trim(res, " ")
	nodeExternalIPs := strings.Split(nodeExternalIP, " ")
	return nodeExternalIPs
}

func FetchIngressIP(kubeconfig string) ([]string, error) {
	cmd := "kubectl get ingress -o jsonpath='{.items[0].status.loadBalancer.ingress[*].ip}' --kubeconfig=" + kubeconfig
	fmt.Println(cmd)
	res, err := RunCommand(cmd)
	if err != nil {
		return nil, err
	}
	ingressIP := strings.Trim(res, " ")
	fmt.Println(ingressIP)
	ingressIPs := strings.Split(ingressIP, " ")
	return ingressIPs, nil
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

func ParsePods(kubeconfig string, print bool) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	podList := ""

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
	}
	if print {
		fmt.Println(podList)
	}
	return pods, nil
}
