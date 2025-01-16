package spegel

import (
	"net"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/rancher/wharfie/pkg/registries"
)

// InjectMirror configures TLS for the registry mirror client, and  adds the mirror address as an endpoint
// to all configured registries.
func (c *Config) InjectMirror(nodeConfig *config.Node) error {
	mirrorAddr := net.JoinHostPort(c.InternalAddress, c.RegistryPort)
	mirrorURL := "https://" + mirrorAddr + "/v2"
	registry := nodeConfig.AgentConfig.Registry

	if registry.Configs == nil {
		registry.Configs = map[string]registries.RegistryConfig{}
	}
	registry.Configs[mirrorAddr] = registries.RegistryConfig{
		TLS: &registries.TLSConfig{
			CAFile:   c.ServerCAFile,
			CertFile: c.ClientCertFile,
			KeyFile:  c.ClientKeyFile,
		},
	}

	if registry.Mirrors == nil {
		registry.Mirrors = map[string]registries.Mirror{}
	}
	for host, mirror := range registry.Mirrors {
		// Don't handle local registry entries
		if !docker.IsLocalhost(host) {
			mirror.Endpoints = append([]string{mirrorURL}, mirror.Endpoints...)
			registry.Mirrors[host] = mirror
		}
	}
	registry.Mirrors[mirrorAddr] = registries.Mirror{
		Endpoints: []string{mirrorURL},
	}

	return nil
}
