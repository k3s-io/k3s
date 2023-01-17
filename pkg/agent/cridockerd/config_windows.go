//go:build windows && !no_cri_dockerd
// +build windows,!no_cri_dockerd

package cridockerd

import (
	"context"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

const socketPrefix = "npipe://"

func setupDockerCRIConfig(ctx context.Context, cfg *config.Node) error {
	return nil
}
