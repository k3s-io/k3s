//go:build linux
// +build linux

package agent

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
)

const (
	containerdSock = "unix:///run/k3s/containerd/containerd.sock"
)

// setupCriCtlConfig creates the crictl config file and populates it
// with the given data from config.
func setupCriCtlConfig(cfg cmds.Agent, nodeConfig *config.Node) error {
	cre := nodeConfig.ContainerRuntimeEndpoint
	if cre == "" {
		cre = containerdSock
	}

	agentConfDir := filepath.Join(cfg.DataDir, "agent", "etc")
	if _, err := os.Stat(agentConfDir); os.IsNotExist(err) {
		if err := os.MkdirAll(agentConfDir, 0700); err != nil {
			return err
		}
	}

	crp := "runtime-endpoint: " + cre + "\n"
	return ioutil.WriteFile(agentConfDir+"/crictl.yaml", []byte(crp), 0600)
}
