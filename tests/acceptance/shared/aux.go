package shared

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
)

// RunCommandHost executes commands on the host
// return the output of it and an error if any
func RunCommandHost(cmds ...string) (string, error) {
	if cmds == nil {
		return "", fmt.Errorf("cmd should not be empty")
	}

	var output, errOut bytes.Buffer
	for _, cmd := range cmds {
		c := exec.Command("bash", "-c", cmd)

		c.Stdout = &output
		c.Stderr = &errOut
		if errOut.Len() > 0 {
			fmt.Println("returning Stderr if not null, this might not be an error",
				errOut.String())
		}

		err := c.Run()
		if err != nil {
			return output.String(), fmt.Errorf("executing command: %s: %w", cmd, err)
		}
	}

	return output.String(), nil
}

// RunCmdOnNode executes a command from within the given ip node
func RunCmdOnNode(cmd string, serverIP string) (string, error) {
	host := serverIP + ":22"
	conn, err := configureSSH(host)
	if err != nil {
		return fmt.Errorf("failed to configure SSH: %v", err).Error(), err
	}

	stdout, stderr, err := runsshCommand(cmd, conn)
	if err != nil {
		return "", fmt.Errorf(
			"command: %s \n failed on run ssh : %s with error %w",
			cmd,
			serverIP,
			err,
		)
	}

	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)

	if stderr != "" && (!strings.Contains(stderr, "error") ||
		!strings.Contains(stderr, "1") ||
		!strings.Contains(stderr, "2")) {
		return stderr, nil
	} else if stderr != "" {
		return fmt.Errorf("\ncommand: %s \n failed with error: %v", cmd, err).Error(), err
	}

	return stdout, err
}

func configureSSH(host string) (*ssh.Client, error) {
	var config *ssh.ClientConfig

	config = &ssh.ClientConfig{
		User: AwsUser,
		Auth: []ssh.AuthMethod{
			publicKey(AccessKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to establish SSH connection to host %s: %v", host, err)
	}

	return conn, nil
}

func runsshCommand(cmd string, conn *ssh.Client) (string, string, error) {
	session, err := conn.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	errssh := session.Run(cmd)
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if errssh != nil {
		return stdoutStr, stderrStr, fmt.Errorf("on command execution: %v", errssh)
	}

	return stdoutStr, stderrStr, nil
}

// GetK3sVersion returns the k3s version
func GetK3sVersion() string {
	ips := FetchNodeExternalIP()
	for _, ip := range ips {
		res, err := RunCmdOnNode("k3s --version", ip)
		if err != nil {
			return err.Error()
		}
		return res
	}
	return ""
}

// AddHelmRepo adds a helm repo to the cluster.
func AddHelmRepo(name, url string) (string, error) {
	InstallHelm := "curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"
	addRepo := fmt.Sprintf("helm repo add %s %s", name, url)
	installRepo := fmt.Sprintf(
		"helm install %s %s/%s -n kube-system", name, name, name)

	nodeExternalIP := FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		_, err := RunCmdOnNode(InstallHelm, ip)
		if err != nil {
			return "", err
		}
	}
	return RunCommandHost(addRepo, installRepo)
}

// CountOfStringInSlice Used to count the pods using prefix passed in the list of pods
func CountOfStringInSlice(str string, pods []Pod) int {
	count := 0
	for _, pod := range pods {
		if strings.Contains(pod.Name, str) {
			count++
		}
	}
	return count
}

// PrintFileContents prints the contents of the file
func PrintFileContents(f ...string) error {
	for _, file := range f {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		fmt.Println(string(content) + "\n")
	}
	return nil
}

// GetBasepath returns the base path of the project
func GetBasepath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "../..")
}

func publicKey(path string) ssh.AuthMethod {
	key, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		panic(err)
	}
	return ssh.PublicKeys(signer)
}

// JoinCommands joins the first command with some argument
func JoinCommands(cmd, arg string) string {
	cmds := strings.Split(cmd, ":")
	firstCmd := cmds[0] + arg

	if len(cmds) > 1 {
		secondCmd := strings.Join(cmds[1:], ",")
		firstCmd += " " + secondCmd
	}

	return firstCmd
}
