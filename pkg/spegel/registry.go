package spegel

import (
	"net"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/rancher/wharfie/pkg/registries"
)

// InjectMirror configures TLS for the registry mirror client, and  adds the mirror address as an endpoint
// to all configured registries.
func (c *Config) InjectMirror(nodeConfig *config.Node) error {
	mirrorAddr := net.JoinHostPort(c.InternalAddress, c.RegistryPort)
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
		if host != "*" {
			mirror.Endpoints = append([]string{"https://" + mirrorAddr}, mirror.Endpoints...)
			registry.Mirrors[host] = mirror
		}
	}

	return nil
}
