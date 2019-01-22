package agent

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/rancher/k3s/pkg/agent/config"
	"github.com/rancher/k3s/pkg/agent/containerd"
	"github.com/rancher/k3s/pkg/agent/flannel"
	"github.com/rancher/k3s/pkg/agent/proxy"
	"github.com/rancher/k3s/pkg/agent/syssetup"
	"github.com/rancher/k3s/pkg/agent/tunnel"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/agent"
	"github.com/rancher/norman/pkg/clientaccess"
	"github.com/sirupsen/logrus"
)

func run(ctx context.Context, cfg cmds.Agent) error {
	nodeConfig := config.Get(ctx, cfg)

	if !nodeConfig.NoFlannel {
		if err := flannel.Prepare(ctx, nodeConfig); err != nil {
			return err
		}
	}

	if nodeConfig.Docker {
		nodeConfig.AgentConfig.RuntimeSocket = ""
	} else {
		if err := containerd.Run(ctx, nodeConfig); err != nil {
			return err
		}
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

func Run(ctx context.Context, cfg cmds.Agent) error {
	cfg.DataDir = filepath.Join(cfg.DataDir, "agent")

	if cfg.ClusterSecret != "" {
		cfg.Token = "K10node:" + cfg.ClusterSecret
	}

	for {
		tmpFile, err := clientaccess.AgentAccessInfoToTempKubeConfig("", cfg.ServerURL, cfg.Token)
		if err != nil {
			logrus.Error(err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
			continue
		}
		os.Remove(tmpFile)
		break
	}

	os.MkdirAll(cfg.DataDir, 0700)
	return run(ctx, cfg)
}
