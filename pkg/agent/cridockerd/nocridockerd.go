//go:build no_cri_dockerd
// +build no_cri_dockerd

package cridockerd

import (
	"context"
	"errors"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

func Run(ctx context.Context, cfg *config.Node) error {
	return errors.New("cri-dockerd disabled at build time")
}
