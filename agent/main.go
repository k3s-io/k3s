package main

import (
	"context"

	"github.com/rancher/norman/signal"
	"github.com/rancher/rio/agent/config"
	"github.com/rancher/rio/agent/containerd"
	"github.com/rancher/rio/agent/flannel"
	"github.com/rancher/rio/agent/proxy"
	"github.com/rancher/rio/agent/syssetup"
	"github.com/rancher/rio/agent/tunnel"
	"github.com/rancher/rio/pkg/daemons/agent"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := run(); err != nil {
		logrus.Fatal(err)
	}
}

func run() error {
	ctx := signal.SigTermCancelContext(context.Background())

	nodeConfig := config.Get()

	if nodeConfig.Docker {
		nodeConfig.AgentConfig.RuntimeSocket = ""
	} else {
		containerd.Run(ctx, nodeConfig)
	}

	if err := syssetup.Configure(); err != nil {
		return err
	}

	if err := tunnel.Setup(nodeConfig); err != nil {
		return err
	}

	if err := proxy.Run(nodeConfig); err != nil {
		return err
	}

	if err := agent.Agent(&nodeConfig.AgentConfig); err != nil {
		return err
	}

	if !nodeConfig.NoFlannel {
		if err := flannel.Run(ctx, nodeConfig); err != nil {
			return err
		}
	}

	<-ctx.Done()
	return ctx.Err()
}
