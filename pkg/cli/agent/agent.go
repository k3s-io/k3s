package agent

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/rancher/k3s/pkg/netutil"
	"github.com/rancher/k3s/pkg/token"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	gspt.SetProcTitle(os.Args[0] + " agent")

	if err := cmds.InitLogging(); err != nil {
		return err
	}
	if os.Getuid() != 0 && runtime.GOOS != "windows" {
		return fmt.Errorf("agent must be ran as root")
	}

	if cmds.AgentConfig.TokenFile != "" {
		token, err := token.ReadFile(cmds.AgentConfig.TokenFile)
		if err != nil {
			return err
		}
		cmds.AgentConfig.Token = token
	}

	if cmds.AgentConfig.Token == "" && cmds.AgentConfig.ClusterSecret != "" {
		cmds.AgentConfig.Token = cmds.AgentConfig.ClusterSecret
	}

	if cmds.AgentConfig.Token == "" {
		return fmt.Errorf("--token is required")
	}

	if cmds.AgentConfig.ServerURL == "" {
		return fmt.Errorf("--server is required")
	}

	if cmds.AgentConfig.FlannelIface != "" && len(cmds.AgentConfig.NodeIP) == 0 {
		cmds.AgentConfig.NodeIP.Set(netutil.GetIPFromInterface(cmds.AgentConfig.FlannelIface))
	}

	logrus.Info("Starting " + version.Program + " agent " + ctx.App.Version)

	dataDir, err := datadir.LocalHome(cmds.AgentConfig.DataDir, cmds.AgentConfig.Rootless)
	if err != nil {
		return err
	}

	cfg := cmds.AgentConfig
	cfg.Debug = ctx.GlobalBool("debug")
	cfg.DataDir = dataDir

	contextCtx := signals.SetupSignalHandler(context.Background())

	return agent.Run(contextCtx, cfg)
}
