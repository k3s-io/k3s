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
	"github.com/rancher/k3s/pkg/agent/netpol"
	"github.com/rancher/k3s/pkg/agent/syssetup"
	"github.com/rancher/k3s/pkg/agent/tunnel"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/agent"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/rootless"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/core"
	corev1 "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	InternalIPLabel = "k3s.io/internal-ip"
	ExternalIPLabel = "k3s.io/external-ip"
	HostnameLabel   = "k3s.io/hostname"
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

	if !nodeConfig.AgentConfig.DisableCCM {
		if err := syncAddressesLabels(ctx, &nodeConfig.AgentConfig); err != nil {
			return err
		}
	}

	if !nodeConfig.AgentConfig.DisableNPC {
		if err := netpol.Run(ctx, nodeConfig); err != nil {
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

func syncAddressesLabels(ctx context.Context, agentConfig *daemonconfig.Agent) error {
	for {
		nodeController, nodeCache, err := startNodeController(ctx, agentConfig)
		if err != nil {
			logrus.Infof("Waiting for kubelet to be ready on node %s: %v", agentConfig.NodeName, err)
			time.Sleep(1 * time.Second)
			continue
		}
		nodeCached, err := nodeCache.Get(agentConfig.NodeName)
		if err != nil {
			logrus.Infof("Waiting for kubelet to be ready on node %s: %v", agentConfig.NodeName, err)
			time.Sleep(1 * time.Second)
			continue
		}
		node := nodeCached.DeepCopy()
		updated := updateLabelMap(ctx, agentConfig, node.Labels)
		if updated {
			_, err = nodeController.Update(node)
			if err == nil {
				logrus.Infof("addresses labels has been set succesfully on node: %s", agentConfig.NodeName)
				break
			}
			logrus.Infof("Failed to update node %s: %v", agentConfig.NodeName, err)
			time.Sleep(1 * time.Second)
			continue
		}
		logrus.Infof("addresses labels has already been set succesfully on node: %s", agentConfig.NodeName)
		return nil
	}
	return nil
}

func startNodeController(ctx context.Context, agentConfig *daemonconfig.Agent) (corev1.NodeController, corev1.NodeCache, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", agentConfig.KubeConfigKubelet)
	if err != nil {
		return nil, nil, err
	}
	coreFactory := core.NewFactoryFromConfigOrDie(restConfig)
	nodeController := coreFactory.Core().V1().Node()
	nodeCache := nodeController.Cache()
	if err := start.All(ctx, 1, coreFactory); err != nil {
		return nil, nil, err
	}

	return nodeController, nodeCache, nil
}

func updateLabelMap(ctx context.Context, agentConfig *daemonconfig.Agent, nodeLabels map[string]string) bool {
	if nodeLabels == nil {
		nodeLabels = make(map[string]string)
	}
	updated := false
	if internalIPLabel, ok := nodeLabels[InternalIPLabel]; !ok || internalIPLabel != agentConfig.NodeIP {
		nodeLabels[InternalIPLabel] = agentConfig.NodeIP
		updated = true
	}
	if hostnameLabel, ok := nodeLabels[HostnameLabel]; !ok || hostnameLabel != agentConfig.NodeName {
		nodeLabels[HostnameLabel] = agentConfig.NodeName
		updated = true
	}
	nodeExternalIP := agentConfig.NodeExternalIP
	if externalIPLabel := nodeLabels[ExternalIPLabel]; externalIPLabel != nodeExternalIP && nodeExternalIP != "" {
		nodeLabels[ExternalIPLabel] = nodeExternalIP
		updated = true
	} else if nodeExternalIP == "" && externalIPLabel != "" {
		delete(nodeLabels, ExternalIPLabel)
		updated = true
	}
	return updated
}
