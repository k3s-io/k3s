package agent

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/agent/config"
	"github.com/rancher/k3s/pkg/agent/containerd"
	"github.com/rancher/k3s/pkg/agent/flannel"
	"github.com/rancher/k3s/pkg/agent/loadbalancer"
	"github.com/rancher/k3s/pkg/agent/syssetup"
	"github.com/rancher/k3s/pkg/agent/tunnel"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/agent"
	"github.com/rancher/k3s/pkg/rootless"
	"github.com/sirupsen/logrus"
)

func run(ctx context.Context, cfg cmds.Agent, lb *loadbalancer.LoadBalancer) error {
	nodeConfig := config.Get(ctx, cfg)

	if !nodeConfig.NoFlannel {
		if err := flannel.Prepare(ctx, nodeConfig); err != nil {
			return err
		}
	}

	if nodeConfig.Docker || nodeConfig.ContainerRuntimeEndpoint != "" {
		nodeConfig.AgentConfig.RuntimeSocket = nodeConfig.ContainerRuntimeEndpoint
		nodeConfig.AgentConfig.CNIPlugin = true
	} else {
		if err := containerd.Run(ctx, nodeConfig); err != nil {
			return err
		}
	}

	if err := syssetup.Configure(); err != nil {
		return err
	}

	if err := tunnel.Setup(ctx, nodeConfig, lb.Update); err != nil {
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
	if err := validate(); err != nil {
		return err
	}

	if cfg.Rootless {
		if err := rootless.Rootless(cfg.DataDir); err != nil {
			return err
		}
	}

	cfg.DataDir = filepath.Join(cfg.DataDir, "agent")
	os.MkdirAll(cfg.DataDir, 0700)

	if cfg.ClusterSecret != "" {
		cfg.Token = "K10node:" + cfg.ClusterSecret
	}

	lb, err := loadbalancer.Setup(ctx, cfg)
	if err != nil {
		return err
	}
	if lb != nil {
		cfg.ServerURL = lb.LoadBalancerServerURL()
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

	return run(ctx, cfg, lb)
}

func validate() error {
	cgroups, err := ioutil.ReadFile("/proc/self/cgroup")
	if err != nil {
		return err
	}

	if !strings.Contains(string(cgroups), "cpuset") {
		logrus.Warn("Failed to find cpuset cgroup, you may need to add \"cgroup_enable=cpuset\" to your linux cmdline (/boot/cmdline.txt on a Raspberry Pi)")
	}

	if !strings.Contains(string(cgroups), "memory") {
		msg := "ailed to find memory cgroup, you may need to add \"cgroup_memory=1 cgroup_enable=memory\" to your linux cmdline (/boot/cmdline.txt on a Raspberry Pi)"
		logrus.Error("F" + msg)
		return errors.New("f" + msg)
	}

	return nil
}
