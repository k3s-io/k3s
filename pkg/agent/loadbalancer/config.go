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
		ServerURL:       lb.scheme + "://" + lb.servers.getDefaultAddress(),
		ServerAddresses: lb.servers.getAddresses(),
	}
	configOut, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return util.WriteFile(lb.configFile, string(configOut))
}

func (lb *LoadBalancer) updateConfig() error {
	if configBytes, err := os.ReadFile(lb.configFile); err == nil {
		config := &lbConfig{}
		if err := json.Unmarshal(configBytes, config); err == nil {
			// if the default server from the config matches our current default,
			// load the rest of the addresses as well.
			if config.ServerURL == lb.scheme+"://"+lb.servers.getDefaultAddress() {
				lb.Update(config.ServerAddresses)
				return nil
			}
		}
	}
	// config didn't exist or used a different default server, write the current config to disk.
	return lb.writeConfig()
}
