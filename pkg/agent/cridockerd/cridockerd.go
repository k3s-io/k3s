package cridockerd

import (
	"context"
	"os"
	"runtime/debug"
	"strings"

	"github.com/Mirantis/cri-dockerd/cmd"
	"github.com/k3s-io/k3s/pkg/cgroups"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	utilsnet "k8s.io/utils/net"
)

func Run(ctx context.Context, cfg *config.Node) error {
	if err := setupDockerCRIConfig(ctx, cfg); err != nil {
		return err
	}

	args := getDockerCRIArgs(cfg)
	command := cmd.NewDockerCRICommand(ctx.Done())
	command.SetArgs(args)
	logrus.Infof("Running cri-dockerd %s", config.ArgString(args))

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logrus.Fatalf("cri-dockerd panic: %s", debug.Stack())
			}
		}()
		logrus.Fatalf("cri-dockerd exited: %v", command.ExecuteContext(ctx))
	}()

	return nil
}

func getDockerCRIArgs(cfg *config.Node) []string {
	argsMap := map[string]string{
		"container-runtime-endpoint": cfg.CRIDockerd.Address,
		"cri-dockerd-root-directory": cfg.CRIDockerd.Root,
	}

	if dualNode, _ := utilsnet.IsDualStackIPs(cfg.AgentConfig.NodeIPs); dualNode {
		argsMap["ipv6-dual-stack"] = "true"
	}

	if logLevel := os.Getenv("CRIDOCKERD_LOG_LEVEL"); logLevel != "" {
		argsMap["log-level"] = logLevel
	}

	if cfg.ContainerRuntimeEndpoint != "" {
		endpoint := cfg.ContainerRuntimeEndpoint
		if !strings.HasPrefix(endpoint, socketPrefix) {
			endpoint = socketPrefix + endpoint
		}
		argsMap["docker-endpoint"] = endpoint
	}

	if cfg.AgentConfig.CNIConfDir != "" {
		argsMap["cni-conf-dir"] = cfg.AgentConfig.CNIConfDir
	}
	if cfg.AgentConfig.CNIBinDir != "" {
		argsMap["cni-bin-dir"] = cfg.AgentConfig.CNIBinDir
	}
	if cfg.AgentConfig.CNIPlugin {
		argsMap["network-plugin"] = "cni"
	}
	if cfg.AgentConfig.PauseImage != "" {
		argsMap["pod-infra-container-image"] = cfg.AgentConfig.PauseImage
	}

	_, runtimeRoot, _ := cgroups.CheckCgroups()
	if runtimeRoot != "" {
		argsMap["runtime-cgroups"] = runtimeRoot
	}

	return config.GetArgs(argsMap, nil)
}
