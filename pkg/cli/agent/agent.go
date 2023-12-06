package agent

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/erikdubbelboer/gspt"
	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/agent"
	"github.com/k3s-io/k3s/pkg/authenticator"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/datadir"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/k3s-io/k3s/pkg/vpn"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	apiauth "k8s.io/apiserver/pkg/authentication/authenticator"
)

func Run(ctx *cli.Context) error {
	// Validate build env
	cmds.MustValidateGolang()

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
	if cmds.AgentConfig.VPNAuthFile != "" {
		cmds.AgentConfig.VPNAuth, err = util.ReadFile(cmds.AgentConfig.VPNAuthFile)
		if err != nil {
			return err
		}
	}

	// Starts the VPN in the agent if config was set up
	if cmds.AgentConfig.VPNAuth != "" {
		err := vpn.StartVPN(cmds.AgentConfig.VPNAuth)
		if err != nil {
			return err
		}
	}

	// Until the agent is run and retrieves config from the server, we won't know
	// if the embedded registry is enabled. If it is not enabled, these are not
	// used as the registry is never started.
	conf := spegel.DefaultRegistry
	conf.Bootstrapper = spegel.NewAgentBootstrapper(cfg.ServerURL, cfg.Token, cfg.DataDir)
	conf.HandlerFunc = func(conf *spegel.Config, router *mux.Router) error {
		// Create and bind a new authenticator using the configured client CA
		authArgs := []string{"--client-ca-file=" + conf.ClientCAFile}
		auth, err := authenticator.FromArgs(authArgs)
		if err != nil {
			return err
		}
		conf.AuthFunc = func() apiauth.Request {
			return auth
		}

		// Create a new server and listen on the configured port
		server := &http.Server{
			Handler: router,
			Addr:    ":" + conf.RegistryPort,
			TLSConfig: &tls.Config{
				ClientAuth: tls.RequestClientCert,
			},
		}
		go func() {
			if err := server.ListenAndServeTLS(conf.ServerCertFile, conf.ServerKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logrus.Fatalf("registry server failed: %v", err)
			}
		}()
		return nil
	}

	return agent.Run(contextCtx, cfg)
}
