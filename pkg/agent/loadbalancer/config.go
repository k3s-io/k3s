package loadbalancer

import (
	"encoding/json"
	"os"

	"github.com/k3s-io/k3s/pkg/agent/util"
)

// lbConfig stores loadbalancer state that should be persisted across restarts.
type lbConfig struct {
	ServerURL       string   `json:"ServerURL"`
	ServerAddresses []string `json:"ServerAddresses"`
}

func (lb *LoadBalancer) writeConfig() error {
	config := &lbConfig{
		ServerURL:       lb.serverURL,
		ServerAddresses: lb.serverAddresses,
	}
	configOut, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFile(lb.configFile, string(configOut))
}

func (lb *LoadBalancer) updateConfig() error {
	writeConfig := true
	if configBytes, err := os.ReadFile(lb.configFile); err == nil {
		config := &lbConfig{}
		if err := json.Unmarshal(configBytes, config); err == nil {
			if config.ServerURL == lb.serverURL {
				writeConfig = false
				lb.setServers(config.ServerAddresses)
			}
		}
	}
	if writeConfig {
		if err := lb.writeConfig(); err != nil {
			return err
		}
	}
	return nil
}
