// +build windows

package containerd

import (
	"context"

	"github.com/rancher/k3s/pkg/daemons/config"
)

// setupContainerdConfig generates the containerd.toml, using a template combined with various
// runtime configurations and registry mirror settings provided by the administrator.
func setupContainerdConfig(ctx context.Context, cfg *config.Node) error {
	// TODO: Create windows config setup.
	return nil
}
