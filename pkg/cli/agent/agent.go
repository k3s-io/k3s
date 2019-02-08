package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/rancher/k3s/pkg/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/norman/pkg/resolvehome"
	"github.com/rancher/norman/signal"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("agent must be ran as root")
	}

	if cmds.AgentConfig.Token == "" && cmds.AgentConfig.ClusterSecret == "" {
		return fmt.Errorf("--token is required")
	}

	if cmds.AgentConfig.ServerURL == "" {
		return fmt.Errorf("--server is required")
	}

	logrus.Infof("Starting k3s agent %s", ctx.App.Version)

	dataDir, err := resolvehome.Resolve(cmds.AgentConfig.DataDir)
	if err != nil {
		return err
	}

	cfg := cmds.AgentConfig
	cfg.Debug = ctx.GlobalBool("debug")
	cfg.DataDir = dataDir

	contextCtx := signal.SigTermCancelContext(context.Background())

	return agent.Run(contextCtx, cfg)
}
