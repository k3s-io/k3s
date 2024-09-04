package containerd

import (
	"bufio"
	"fmt"
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
	if err := cleanContainerdHosts(cfg.Containerd.Registry, hosts); err != nil {
		return err
	}

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

// cleanContainerdHosts removes any registry host config dirs containing a hosts.toml file
// with a header that indicates it was created by k3s, or directories where a hosts.toml
// is about to be written.  Unmanaged directories not containing this file, or containing
// a file without the header, are left alone.
func cleanContainerdHosts(dir string, hosts HostConfigs) error {
	// clean directories for any registries that we are about to generate a hosts.toml for
	for host := range hosts {
		hostsDir := filepath.Join(dir, host)
		os.RemoveAll(hostsDir)
	}

	// clean directories that contain a hosts.toml with a header indicating it was  created by k3s
	ents, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, ent := range ents {
		if !ent.IsDir() {
			continue
		}
		hostsFile := filepath.Join(dir, ent.Name(), "hosts.toml")
		file, err := os.Open(hostsFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		line, err := bufio.NewReader(file).ReadString('\n')
		if err != nil {
			continue
		}
		if line == templates.HostsTomlHeader {
			hostsDir := filepath.Join(dir, ent.Name())
			os.RemoveAll(hostsDir)
		}
	}

	return nil
}

// getHostConfigs merges the registry mirrors/configs into HostConfig template structs
func getHostConfigs(registry *registries.Registry, noDefaultEndpoint bool, mirrorAddr string) HostConfigs {
	hosts := map[string]templates.HostConfig{}

	// create config for default endpoints
	for host, config := range registry.Configs {
		if c, err := defaultHostConfig(host, mirrorAddr, config); err != nil {
			logrus.Errorf("Failed to generate config for registry %s: %v", host, err)
		} else {
			if host == "*" {
				host = "_default"
			}
			hosts[host] = *c
		}
	}

	// create endpoints for mirrors
	for host, mirror := range registry.Mirrors {
		// create the default config, if it wasn't explicitly mentioned in the config section
		config, ok := hosts[host]
		if !ok {
			if c, err := defaultHostConfig(host, mirrorAddr, configForHost(registry.Configs, host)); err != nil {
				logrus.Errorf("Failed to generate config for registry %s: %v", host, err)
				continue
			} else {
				if noDefaultEndpoint {
					c.Default = nil
				} else if host == "*" {
					c.Default = &templates.RegistryEndpoint{URL: &url.URL{}}
				}
				config = *c
			}
		}

		// track which endpoints we've already seen to avoid creating duplicates
		seenEndpoint := map[string]bool{}

		// TODO: rewrites are currently copied from the mirror settings into each endpoint.
		// In the future, we should allow for per-endpoint rewrites, instead of expecting
		// all mirrors to have the same structure. This will require changes to the registries.yaml
		// structure, which is defined in rancher/wharfie.
		for i, endpoint := range mirror.Endpoints {
			registryName, url, override, err := normalizeEndpointAddress(endpoint, mirrorAddr)
			if err != nil {
				logrus.Warnf("Ignoring invalid endpoint URL %d=%s for %s: %v", i, endpoint, host, err)
			} else if _, ok := seenEndpoint[url.String()]; ok {
				logrus.Warnf("Skipping duplicate endpoint URL %d=%s for %s", i, endpoint, host)
			} else {
				seenEndpoint[url.String()] = true
				var rewrites map[string]string
				// Do not apply rewrites to the embedded registry endpoint
				if url.Host != mirrorAddr {
					rewrites = mirror.Rewrites
				}
				ep := templates.RegistryEndpoint{
					Config:       configForHost(registry.Configs, registryName),
					Rewrites:     rewrites,
					OverridePath: override,
					URL:          url,
				}
				if i+1 == len(mirror.Endpoints) && endpointURLEqual(config.Default, &ep) {
					// if the last endpoint is the default endpoint, move it there
					config.Default = &ep
				} else {
					config.Endpoints = append(config.Endpoints, ep)
				}
			}
		}

		if host == "*" {
			host = "_default"
		}
		hosts[host] = config
	}

	// Clean up hosts and default endpoints where resulting config leaves only defaults
	for host, config := range hosts {
		// if this host has no endpoints and the default has no config, delete this host
		if len(config.Endpoints) == 0 && !endpointHasConfig(config.Default) {
			delete(hosts, host)
		}
	}

	return hosts
}

// normalizeEndpointAddress normalizes the endpoint address.
// If successful, it returns the registry name, URL, and a bool indicating if the endpoint path should be overridden.
// If unsuccessful, an error is returned.
// Scheme and hostname logic should match containerd:
// https://github.com/containerd/containerd/blob/v1.7.13/remotes/docker/config/hosts.go#L99-L131
func normalizeEndpointAddress(endpoint, mirrorAddr string) (string, *url.URL, bool, error) {
	// Ensure that the endpoint address has a scheme so that the URL is parsed properly
	if !strings.Contains(endpoint, "://") {
		endpoint = "//" + endpoint
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return "", nil, false, err
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
	registry := endpointURL.Host
	endpointURL.Host, _ = docker.DefaultHost(registry)
	// This is the reverse of the DefaultHost normalization
	if endpointURL.Host == "registry-1.docker.io" {
		registry = "docker.io"
	}

	switch endpointURL.Path {
	case "", "/", "/v2":
		// If the path is empty, /, or /v2, use the default path.
		endpointURL.Path = "/v2"
		return registry, endpointURL, false, nil
	}

	return registry, endpointURL, true, nil
}

func defaultHostConfig(host, mirrorAddr string, config registries.RegistryConfig) (*templates.HostConfig, error) {
	_, url, _, err := normalizeEndpointAddress(host, mirrorAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL %s for %s: %v", host, host, err)
	}
	if host == "*" {
		url = nil
	}
	return &templates.HostConfig{
		Program: version.Program,
		Default: &templates.RegistryEndpoint{
			URL:    url,
			Config: config,
		},
	}, nil
}

func configForHost(configs map[string]registries.RegistryConfig, host string) registries.RegistryConfig {
	// check for config under modified hostname. If the hostname is unmodified, or there is no config for
	// the modified hostname, return the config for the default hostname.
	if h, _ := docker.DefaultHost(host); h != host {
		if c, ok := configs[h]; ok {
			return c
		}
	}
	return configs[host]
}

// endpointURLEqual compares endpoint URL strings
func endpointURLEqual(a, b *templates.RegistryEndpoint) bool {
	var au, bu string
	if a != nil && a.URL != nil {
		au = a.URL.String()
	}
	if b != nil && b.URL != nil {
		bu = b.URL.String()
	}
	return au == bu
}

func endpointHasConfig(ep *templates.RegistryEndpoint) bool {
	if ep != nil {
		return ep.OverridePath || ep.Config.Auth != nil || ep.Config.TLS != nil || len(ep.Rewrites) > 0
	}
	return false
}
