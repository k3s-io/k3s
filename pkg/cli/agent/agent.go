package agent

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/agent"
	"github.com/k3s-io/k3s/pkg/agent/https"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/datadir"
	k3smetrics "github.com/k3s-io/k3s/pkg/metrics"
	"github.com/k3s-io/k3s/pkg/proctitle"
	"github.com/k3s-io/k3s/pkg/profile"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/permissions"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/k3s-io/k3s/pkg/vpn"
	"github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	// Validate build env
	cmds.MustValidateGolang()

	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	proctitle.SetProcTitle(os.Args[0] + " agent")

	// Evacuate cgroup v2 before doing anything else that may fork.
	if err := cmds.EvacuateCgroup2(); err != nil {
		return err
	}

	// Initialize logging, and subprocess reaping if necessary.
	// Log output redirection and subprocess reaping both require forking.
	if err := cmds.InitLogging(); err != nil {
		return err
	}

	if !cmds.AgentConfig.Rootless {
		if err := permissions.IsPrivileged(); err != nil {
			return errors.Wrap(err, "agent requires additional privilege if not run with --rootless")
		}
	}

	if cmds.AgentConfig.TokenFile != "" {
		token, err := util.ReadFile(cmds.AgentConfig.TokenFile)
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
		ip, err := util.GetIPFromInterface(cmds.AgentConfig.FlannelIface)
		if err != nil {
			return err
		}
		cmds.AgentConfig.NodeIP.Set(ip)
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

	go cmds.WriteCoverage(contextCtx)
	if cfg.VPNAuthFile != "" {
		cfg.VPNAuth, err = util.ReadFile(cfg.VPNAuthFile)
		if err != nil {
			return err
		}
	}

	// Starts the VPN in the agent if config was set up
	if cfg.VPNAuth != "" {
		err := vpn.StartVPN(cfg.VPNAuth)
		if err != nil {
			return err
		}
	}

	// Until the agent is run and retrieves config from the server, we won't know
	// if the embedded registry is enabled. If it is not enabled, these are not
	// used as the registry is never started.
	registry := spegel.DefaultRegistry
	registry.Bootstrapper = spegel.NewAgentBootstrapper(cfg.ServerURL, cfg.Token, cfg.DataDir)
	registry.Router = func(ctx context.Context, nodeConfig *config.Node) (*mux.Router, error) {
		return https.Start(ctx, nodeConfig, nil)
	}

	// same deal for metrics - these are not used if the extra metrics listener is not enabled.
	metrics := k3smetrics.DefaultMetrics
	metrics.Router = func(ctx context.Context, nodeConfig *config.Node) (*mux.Router, error) {
		return https.Start(ctx, nodeConfig, nil)
	}

	// and for pprof as well
	pprof := profile.DefaultProfiler
	pprof.Router = func(ctx context.Context, nodeConfig *config.Node) (*mux.Router, error) {
		return https.Start(ctx, nodeConfig, nil)
	}

	return agent.Run(contextCtx, cfg)
}
