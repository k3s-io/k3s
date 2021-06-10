package templates

import (
	"github.com/rancher/wharfie/pkg/registries"

	"github.com/rancher/k3s/pkg/daemons/config"
)

type ContainerdConfig struct {
	NodeConfig            *config.Node
	DisableCgroup         bool
	IsRunningInUserNS     bool
	PrivateRegistryConfig *registries.Registry
}
