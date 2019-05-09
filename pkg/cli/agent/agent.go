package agent

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/rancher/wrangler/pkg/signals"

	"github.com/rancher/k3s/pkg/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/datadir"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func readToken(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	for {
		tokenBytes, err := ioutil.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(tokenBytes)), nil
		} else if os.IsNotExist(err) {
			logrus.Infof("Waiting for %s to be available\n", path)
			time.Sleep(2 * time.Second)
		} else {
			return "", err
		}
	}
}

func Run(ctx *cli.Context) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("agent must be ran as root")
	}

	if cmds.AgentConfig.TokenFile != "" {
		token, err := readToken(cmds.AgentConfig.TokenFile)
		if err != nil {
			return err
		}
		cmds.AgentConfig.Token = token
	}

	if cmds.AgentConfig.Token == "" && cmds.AgentConfig.ClusterSecret == "" {
		return fmt.Errorf("--token is required")
	}

	if cmds.AgentConfig.ServerURL == "" {
		return fmt.Errorf("--server is required")
	}

	logrus.Infof("Starting k3s agent %s", ctx.App.Version)

	dataDir, err := datadir.LocalHome(cmds.AgentConfig.DataDir, cmds.AgentConfig.Rootless)
	if err != nil {
		return err
	}

	cfg := cmds.AgentConfig
	cfg.Debug = ctx.GlobalBool("debug")
	cfg.DataDir = dataDir
	cfg.Labels = append(cfg.Labels, "node-role.kubernetes.io/worker=true")

	contextCtx := signals.SetupSignalHandler(context.Background())

	return agent.Run(contextCtx, cfg)
}
