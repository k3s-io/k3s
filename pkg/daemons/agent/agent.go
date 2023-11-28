package agent

import (
	"context"
	"math/rand"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/agent/proxy"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/sirupsen/logrus"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
)

func Agent(ctx context.Context, nodeConfig *daemonconfig.Node, proxy proxy.Proxy) error {
	rand.Seed(time.Now().UTC().UnixNano())
	logsapi.ReapplyHandling = logsapi.ReapplyHandlingIgnoreUnchanged
	logs.InitLogs()
	defer logs.FlushLogs()
	if err := startKubelet(ctx, &nodeConfig.AgentConfig); err != nil {
		return err
	}

	go func() {
		if !config.KubeProxyDisabled(ctx, nodeConfig, proxy) {
			if err := startKubeProxy(ctx, &nodeConfig.AgentConfig); err != nil {
				logrus.Fatalf("Failed to start kube-proxy: %v", err)
			}
		}
	}()

	return nil
}

func startKubeProxy(ctx context.Context, cfg *daemonconfig.Agent) error {
	argsMap := kubeProxyArgs(cfg)
	args := daemonconfig.GetArgs(argsMap, cfg.ExtraKubeProxyArgs)
	logrus.Infof("Running kube-proxy %s", daemonconfig.ArgString(args))
	return executor.KubeProxy(ctx, args)
}

func startKubelet(ctx context.Context, cfg *daemonconfig.Agent) error {
	argsMap := kubeletArgs(cfg)

	args := daemonconfig.GetArgs(argsMap, cfg.ExtraKubeletArgs)
	logrus.Infof("Running kubelet %s", daemonconfig.ArgString(args))

	return executor.Kubelet(ctx, args)
}
