//go:build !no_cri_dockerd
// +build !no_cri_dockerd

package cridockerd

import (
	"context"
	"errors"
	"os"
	"runtime/debug"
	"strings"

	"github.com/Mirantis/cri-dockerd/cmd"
	"github.com/Mirantis/cri-dockerd/cmd/version"

	"github.com/k3s-io/k3s/pkg/agent/cri"
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
	logrus.Infof("cri-dockerd version %s", version.FullVersion())

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("stack", string(debug.Stack())).Fatalf("cri-dockerd panic: %v", err)
			}
		}()
		err := command.ExecuteContext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("cri-dockerd exited: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	return cri.WaitForService(ctx, cfg.CRIDockerd.Address, "cri-dockerd")
}

func getDockerCRIArgs(cfg *config.Node) []string {
	argsMap := map[string]string{
		"container-runtime-endpoint": cfg.CRIDockerd.Address,
		"cri-dockerd-root-directory": cfg.CRIDockerd.Root,
		"streaming-bind-addr":        "127.0.0.1:10010",
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
