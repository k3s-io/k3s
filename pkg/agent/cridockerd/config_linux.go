//go:build linux && !no_cri_dockerd
// +build linux,!no_cri_dockerd

package cridockerd

import (
	"context"
	"strings"

	"github.com/docker/docker/client"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	pkgerrors "github.com/pkg/errors"
)

const socketPrefix = "unix://"

func setupDockerCRIConfig(ctx context.Context, cfg *config.Node) error {
	clientOpts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if cfg.ContainerRuntimeEndpoint != "" {
		host := cfg.ContainerRuntimeEndpoint
		if !strings.HasPrefix(host, socketPrefix) {
			host = socketPrefix + host
		}
		clientOpts = append(clientOpts, client.WithHost(host))
	}
	c, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to create docker client")
	}
	i, err := c.Info(ctx)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to get docker runtime info")
	}
	// note: this mutatation of the passed agent.Config is later used to set the
	// kubelet's cgroup-driver flag. This may merit moving to somewhere else in order
	// to avoid mutating the configuration while setting up the docker CRI.
	cfg.AgentConfig.Systemd = i.CgroupDriver == "systemd"
	return nil
}
