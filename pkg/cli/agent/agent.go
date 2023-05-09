package agent

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/erikdubbelboer/gspt"
	"github.com/k3s-io/k3s/pkg/agent"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/token"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	gspt.SetProcTitle(os.Args[0] + " agent")

	// Evacuate cgroup v2 before doing anything else that may fork.
	if err := cmds.EvacuateCgroup2(); err != nil {
		return err
	}

	// Initialize logging, and subprocess reaping if necessary.
	// Log output redirection and subprocess reaping both require forking.
	if err := cmds.InitLogging(); err != nil {
		return err
	}

	if runtime.GOOS != "windows" && os.Getuid() != 0 && !cmds.AgentConfig.Rootless {
		return fmt.Errorf("agent must be run as root, or with --rootless")
	}

	if cmds.AgentConfig.TokenFile != "" {
		token, err := token.ReadFile(cmds.AgentConfig.TokenFile)
		if err != nil {
			return err
		}
		cmds.AgentConfig.Token = token
	}

	clientKubeletCert := filepath.Join(cmds.AgentConfig.DataDir, "agent", "client-kubelet.crt")
	clientKubeletKey := filepath.Join(cmds.AgentConfig.DataDir, "agent", "client-kubelet.key")
	_, err := tls.LoadX509KeyPair(clientKubeletCert, clientKubeletKey)

	if err != nil && cmds.AgentConfig.Token == "" {
		return fmt.Errorf("--token is required")
	}

	if cmds.AgentConfig.ServerURL == "" {
		return fmt.Errorf("--server is required")
	}

	if cmds.AgentConfig.FlannelIface != "" && len(cmds.AgentConfig.NodeIP) == 0 {
		cmds.AgentConfig.NodeIP.Set(util.GetIPFromInterface(cmds.AgentConfig.FlannelIface))
	}

	logrus.Info("Starting " + version.Program + " agent " + ctx.App.Version)

	dataDir, err := datadir.LocalHome(cmds.AgentConfig.DataDir, cmds.AgentConfig.Rootless)
	if err != nil {
		return err
	}

	cfg := cmds.AgentConfig
	cfg.Debug = ctx.GlobalBool("debug")
	cfg.DataDir = dataDir

	contextCtx := signals.SetupSignalContext()

	return agent.Run(contextCtx, cfg)
}
