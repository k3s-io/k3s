//go:build windows
// +build windows

package cridockerd

import (
	"context"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

const socketPrefix = "npipe://"

func setupDockerCRIConfig(ctx context.Context, cfg *config.Node) error {
	return nil
}
