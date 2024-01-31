package containerd

import (
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/remotes/docker"
	"github.com/k3s-io/k3s/pkg/agent/templates"
	util2 "github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/sirupsen/logrus"
)

type HostConfigs map[string]templates.HostConfig

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
	mirrorAddr := net.JoinHostPort(spegel.DefaultRegistry.InternalAddress, spegel.DefaultRegistry.RegistryPort)
	hosts := getHostConfigs(containerdConfig.PrivateRegistryConfig, containerdConfig.NoDefaultEndpoint, mirrorAddr)

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

// getHostConfigs merges the registry mirrors/configs into HostConfig template structs
func getHostConfigs(registry *registries.Registry, noDefaultEndpoint bool, mirrorAddr string) HostConfigs {
	hosts := map[string]templates.HostConfig{}

	// create endpoints for mirrors
	for host, mirror := range registry.Mirrors {
		config := templates.HostConfig{
			Program: version.Program,
		}
		if uri, _, err := normalizeEndpointAddress(host, mirrorAddr); err == nil {
			config.DefaultEndpoint = uri.String()
		}

		// TODO: rewrites are currently copied from the mirror settings into each endpoint.
		// In the future, we should allow for per-endpoint rewrites, instead of expecting
		// all mirrors to have the same structure. This will require changes to the registries.yaml
		// structure, which is defined in rancher/wharfie.
		for _, endpoint := range mirror.Endpoints {
			uri, override, err := normalizeEndpointAddress(endpoint, mirrorAddr)
			if err != nil {
				logrus.Warnf("Ignoring invalid endpoint URL %s for %s: %v", endpoint, host, err)
			} else {
				var rewrites map[string]string
				// Do not apply rewrites to the embedded registry endpoint
				if uri.Host != mirrorAddr {
					rewrites = mirror.Rewrites
				}
				config.Endpoints = append(config.Endpoints, templates.RegistryEndpoint{
					Config:       registry.Configs[uri.Host],
					Rewrites:     rewrites,
					OverridePath: override,
					URI:          uri.String(),
				})
			}
		}

		if host == "*" {
			host = "_default"
		}
		hosts[host] = config
	}

	// create endpoints for registries using default endpoints
	for host, registry := range registry.Configs {
		config, ok := hosts[host]
		if !ok {
			config = templates.HostConfig{
				Program: version.Program,
			}
			if uri, _, err := normalizeEndpointAddress(host, mirrorAddr); err == nil {
				config.DefaultEndpoint = uri.String()
			}
		}
		// If there is config for this host but no endpoints, inject the config for the default endpoint.
		if len(config.Endpoints) == 0 {
			uri, _, err := normalizeEndpointAddress(host, mirrorAddr)
			if err != nil {
				logrus.Warnf("Ignoring invalid endpoint URL %s for %s: %v", host, host, err)
			} else {
				config.Endpoints = append(config.Endpoints, templates.RegistryEndpoint{
					Config: registry,
					URI:    uri.String(),
				})
			}
		}

		if host == "*" {
			host = "_default"
		}
		hosts[host] = config
	}

	// Clean up hosts and default endpoints where resulting config leaves only defaults
	for host, config := range hosts {
		// if default endpoint is disabled, or this is the wildcard host, delete the default endpoint
		if noDefaultEndpoint || host == "_default" {
			config.DefaultEndpoint = ""
			hosts[host] = config
		}
		if l := len(config.Endpoints); l > 0 {
			if ep := config.Endpoints[l-1]; ep.URI == config.DefaultEndpoint {
				// if the last endpoint is the default endpoint
				if ep.Config.Auth == nil && ep.Config.TLS == nil && len(ep.Rewrites) == 0 {
					// if has no config, delete this host to use the default config
					delete(hosts, host)
				} else {
					// if it has config, delete the default endpoint
					config.DefaultEndpoint = ""
					hosts[host] = config
				}
			}
		} else {
			// if this host has no endpoints, delete this host to use the default config
			delete(hosts, host)
		}
	}

	return hosts
}

// normalizeEndpointAddress normalizes the endpoint address.
// If successful, it returns the URL, and a bool indicating if the endpoint path should be overridden.
// If unsuccessful, an error is returned.
// Scheme and hostname logic should match containerd:
// https://github.com/containerd/containerd/blob/v1.7.13/remotes/docker/config/hosts.go#L99-L131
func normalizeEndpointAddress(endpoint, mirrorAddr string) (*url.URL, bool, error) {
	// Ensure that the endpoint address has a scheme so that the URL is parsed properly
	if !strings.Contains(endpoint, "://") {
		endpoint = "//" + endpoint
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, false, err
	}
	port := endpointURL.Port()

	// set default scheme, if not provided
	if endpointURL.Scheme == "" {
		// localhost on odd ports defaults to http, unless it's the embedded mirror
		if docker.IsLocalhost(endpointURL.Host) && port != "" && port != "443" && endpointURL.Host != mirrorAddr {
			endpointURL.Scheme = "http"
		} else {
			endpointURL.Scheme = "https"
		}
	}
	endpointURL.Host, _ = docker.DefaultHost(endpointURL.Host)

	switch endpointURL.Path {
	case "", "/", "/v2":
		// If the path is empty, /, or /v2, use the default path.
		endpointURL.Path = "/v2"
		return endpointURL, false, nil
	}

	return endpointURL, true, nil
}
