package util

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
)

// RunCommandHost executes commands on the host
// return the output of it and an error if any
func RunCommandHost(cmd ...string) (string, error) {
	var output bytes.Buffer
	for _, cmd := range cmd {
		c := exec.Command("bash", "-c", cmd)

		stdoutPipe, err := c.StdoutPipe()
		if err != nil {
			return "", err
		}
		stderrPipe, err := c.StderrPipe()
		if err != nil {
			return "", err
		}

		err = c.Start()
		if err != nil {
			return "", err
		}

		_, err = io.Copy(&output, stdoutPipe)
		if err != nil {
			return "", err
		}

		_, err = io.Copy(&output, stderrPipe)
		if err != nil {
			return "", err
		}

		err = c.Wait()
		if err != nil {
			return output.String(), fmt.Errorf("error executing command: %s: %w", cmd, err)
		}
	}

	return output.String(), nil
}

// RunCmdOnNode executes a command from within the given ip node
func RunCmdOnNode(cmd string, serverIP string) (string, error) {
	host := serverIP + ":22"
	conn := configureSSH(host)

	stdout, stderr, err := runsshCommand(cmd, conn)
	if err != nil {
		return fmt.Errorf(
			"Command: %s \n failed on Node: %s with error: %w",
			cmd,
			serverIP,
			err,
		).Error(), nil
	}

	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)

	if stderr != "" && (!strings.Contains(stderr, "error") ||
		!strings.Contains(stderr, "1") ||
		!strings.Contains(stderr, "2")) {
		return stderr, nil
	} else if stderr != "" {
		log.Fatalf("Command: %s \n failed with error: %v", cmd, stderr)
	}

	return stdout, err
}

// GetK3sVersion returns the k3s version with commit hash
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

// configureSSH configures the SSH connection to the host
func configureSSH(host string) *ssh.Client {
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
		log.Fatalf("Failed to establish SSH connection to host %s: %v", host, err)
	}

	return conn
}

// AddHelmRepo adds a helm repo to the cluster.
func AddHelmRepo(repoName, url string) (string, error) {
	addRepo := fmt.Sprintf("helm repo add %s %s", repoName, url)
	installRepo := fmt.Sprintf(
		"helm install %s %s/%s -n kube-system",
		repoName,
		repoName,
		repoName,
	)

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
		return stdoutStr, stderrStr, fmt.Errorf("error on command execution: %v", errssh)
	}

	return stdoutStr, stderrStr, nil
}
