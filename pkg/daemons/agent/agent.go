package agent

import (
	"context"
	"math/rand"
	"os"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/agent/proxy"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/sirupsen/logrus"
	"k8s.io/component-base/logs"
	_ "k8s.io/component-base/metrics/prometheus/restclient" // for client metric registration
	_ "k8s.io/component-base/metrics/prometheus/version"    // for version metric registration
)

const (
	unixPrefix    = "unix://"
	windowsPrefix = "npipe://"
)

func Agent(ctx context.Context, nodeConfig *daemonconfig.Node, proxy proxy.Proxy) error {
	rand.Seed(time.Now().UTC().UnixNano())

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

// ImageCredProvAvailable checks to see if the kubelet image credential provider bin dir and config
// files exist and are of the correct types. This is exported so that it may be used by downstream projects.
func ImageCredProvAvailable(cfg *daemonconfig.Agent) bool {
	if info, err := os.Stat(cfg.ImageCredProvBinDir); err != nil || !info.IsDir() {
		logrus.Debugf("Kubelet image credential provider bin directory check failed: %v", err)
		return false
	}
	if info, err := os.Stat(cfg.ImageCredProvConfig); err != nil || info.IsDir() {
		logrus.Debugf("Kubelet image credential provider config file check failed: %v", err)
		return false
	}
	return true
}
