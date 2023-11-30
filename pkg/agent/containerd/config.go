package containerd

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/k3s-io/k3s/pkg/agent/templates"
	util2 "github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

// writeContainerdConfig renders and saves config.toml from the filled template
func writeContainerdConfig(cfg *config.Node, containerdConfig templates.ContainerdConfig) error {
	var containerdTemplate string
	containerdTemplateBytes, err := os.ReadFile(cfg.Containerd.Template)
	if err == nil {
		logrus.Infof("Using containerd template at %s", cfg.Containerd.Template)
		containerdTemplate = string(containerdTemplateBytes)
	} else if os.IsNotExist(err) {
		containerdTemplate = templates.ContainerdConfigTemplate
	} else {
		return err
	}
	parsedTemplate, err := templates.ParseTemplateFromConfig(containerdTemplate, containerdConfig)
	if err != nil {
		return err
	}

	return util2.WriteFile(cfg.Containerd.Config, parsedTemplate)
}

// writeContainerdHosts merges registry mirrors/configs, and renders and saves hosts.toml from the filled template
func writeContainerdHosts(cfg *config.Node, containerdConfig templates.ContainerdConfig) error {
	registry := containerdConfig.PrivateRegistryConfig
	hosts := map[string]templates.HostConfig{}

	for host, mirror := range registry.Mirrors {
		defaultHost, _ := docker.DefaultHost(host)
		config := templates.HostConfig{
			Host:    defaultHost,
			Program: version.Program,
		}
		if host == "*" {
			host = "_default"
			config.Host = ""
		} else if containerdConfig.NoDefaultEndpoint {
			config.Host = ""
		}
		// TODO: rewrites are currently copied from the mirror settings into each endpoint.
		// In the future, we should allow for per-endpoint rewrites, instead of expecting
		// all mirrors to have the same structure. This will require changes to the registries.yaml
		// structure, which is defined in rancher/wharfie.
		for _, endpoint := range mirror.Endpoints {
			if endpointURL, err := url.Parse(endpoint); err == nil {
				config.Endpoints = append(config.Endpoints, templates.RegistryEndpoint{
					OverridePath: endpointURL.Path != "" && endpointURL.Path != "/" && !strings.HasSuffix(endpointURL.Path, "/v2"),
					Config:       registry.Configs[endpointURL.Host],
					Rewrites:     mirror.Rewrites,
					URI:          endpoint,
				})
			}
		}
		hosts[host] = config
	}

	for host, registry := range registry.Configs {
		config, ok := hosts[host]
		if !ok {
			config = templates.HostConfig{
				Program: version.Program,
			}
		}
		if len(config.Endpoints) == 0 {
			config.Endpoints = []templates.RegistryEndpoint{
				{
					Config: registry,
					URI:    "https://" + host,
				},
			}
		}
		hosts[host] = config
	}

	// Clean up previous configuration templates
	os.RemoveAll(cfg.Containerd.Registry)

	// Write out new templates
	for host, config := range hosts {
		hostDir := filepath.Join(cfg.Containerd.Registry, host)
		hostsFile := filepath.Join(hostDir, "hosts.toml")
		hostsTemplate, err := templates.ParseHostsTemplateFromConfig(templates.HostsTomlTemplate, config)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(hostDir, 0700); err != nil {
			return err
		}
		if err := util2.WriteFile(hostsFile, hostsTemplate); err != nil {
			return err
		}
	}

	return nil
}
