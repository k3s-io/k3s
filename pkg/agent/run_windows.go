// +build windows

package agent

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
)

const (
	dockershimSock = "npipe:////./pipe/docker_engine"
	containerdSock = "npipe:////./pipe/containerd-containerd"
)

// setupCriCtlConfig creates the crictl config file and populates it
// with the given data from config.
func setupCriCtlConfig(cfg cmds.Agent, nodeConfig *config.Node) error {
	cre := nodeConfig.ContainerRuntimeEndpoint
	if cre == "" || strings.HasPrefix(cre, "npipe:") {
		switch {
		case cfg.Docker:
			cre = dockershimSock
		default:
			cre = containerdSock
		}
	} else {
		cre = containerdSock
	}
	agentConfDir := filepath.Join(cfg.DataDir, "agent", "etc")
	if _, err := os.Stat(agentConfDir); os.IsNotExist(err) {
		if err := os.MkdirAll(agentConfDir, 0700); err != nil {
			return err
		}
	}

	crp := "runtime-endpoint: " + cre + "\n"
	return ioutil.WriteFile(filepath.Join(agentConfDir, "crictl.yaml"), []byte(crp), 0600)
}
