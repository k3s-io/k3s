package templates

import (
	"github.com/rancher/wharfie/pkg/registries"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

type ContainerdRuntimeConfig struct {
	RuntimeType string
	BinaryName  string
}

type ContainerdConfig struct {
	NodeConfig            *config.Node
	DisableCgroup         bool
	SystemdCgroup         bool
	IsRunningInUserNS     bool
	PrivateRegistryConfig *registries.Registry
	ExtraRuntimes         map[string]ContainerdRuntimeConfig
}
