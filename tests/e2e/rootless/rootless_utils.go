package rootless

import (
	"fmt"
	"os"
	"strings"

	"github.com/k3s-io/k3s/tests/e2e"
)

// RunCmdOnRootlessNode executes a command from within the given node as user vagrant
func RunCmdOnRootlessNode(cmd string, nodename string) (string, error) {
	injectEnv := ""
	if _, ok := os.LookupEnv("E2E_GOCOVER"); ok && strings.HasPrefix(cmd, "k3s") {
		injectEnv = "GOCOVERDIR=/tmp/k3scov "
	}
	runcmd := "vagrant ssh " + nodename + " -c \"" + injectEnv + cmd + "\""
	out, err := e2e.RunCommand(runcmd)
	if err != nil {
		return out, fmt.Errorf("failed to run command: %s on node %s: %s, %v", cmd, nodename, out, err)
	}
	return out, nil
}

func GenRootlessKubeconfigFile(serverName string) (string, error) {
	kubeConfig, err := RunCmdOnRootlessNode("cat /home/vagrant/.kube/k3s.yaml", serverName)
	if err != nil {
		return "", err
	}
	vNode := e2e.VagrantNode(serverName)
	nodeIP, err := vNode.FetchNodeExternalIP()
	if err != nil {
		return "", err
	}
	kubeConfig = strings.Replace(kubeConfig, "127.0.0.1", nodeIP, 1)
	kubeConfigFile := fmt.Sprintf("kubeconfig-%s", serverName)
	if err := os.WriteFile(kubeConfigFile, []byte(kubeConfig), 0644); err != nil {
		return "", err
	}
	if err := os.Setenv("E2E_KUBECONFIG", kubeConfigFile); err != nil {
		return "", err
	}
	return kubeConfigFile, nil
}
