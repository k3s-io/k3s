package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/reexec"
	"github.com/natefinch/lumberjack"
	"github.com/rancher/k3s/pkg/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/norman/signal"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	_ "github.com/mattn/go-sqlite3" // ensure we have sqlite
)

func setupLogging(app *cli.Context) {
	if !app.GlobalBool("debug") {
		flag.Set("stderrthreshold", "3")
		flag.Set("alsologtostderr", "false")
		flag.Set("logtostderr", "false")
	}
}

func runWithLogging(app *cli.Context, cfg *cmds.Server) error {
	l := &lumberjack.Logger{
		Filename:   cfg.Log,
		MaxSize:    50,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	args := append([]string{"k3s"}, os.Args[1:]...)
	cmd := reexec.Command(args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "_RIO_REEXEC_=true")
	cmd.Stderr = l
	cmd.Stdout = l
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func Run(app *cli.Context) error {
	return run(app, &cmds.ServerConfig)
}

func run(app *cli.Context, cfg *cmds.Server) error {
	if cfg.Log != "" && os.Getenv("_RIO_REEXEC_") == "" {
		return runWithLogging(app, cfg)
	}

	setupLogging(app)

	if !cfg.DisableAgent && os.Getuid() != 0 {
		return fmt.Errorf("must run as root unless --disable-agent is specified")
	}

	serverConfig := server.Config{}
	serverConfig.ControlConfig.ClusterSecret = cfg.ClusterSecret
	serverConfig.ControlConfig.DataDir = cfg.DataDir
	serverConfig.ControlConfig.KubeConfigOutput = cfg.KubeConfigOutput
	serverConfig.ControlConfig.KubeConfigMode = cfg.KubeConfigMode
	serverConfig.TLSConfig.HTTPSPort = cfg.HTTPSPort
	serverConfig.TLSConfig.HTTPPort = cfg.HTTPPort

	// TODO: support etcd
	serverConfig.ControlConfig.NoLeaderElect = true

	for _, noDeploy := range app.StringSlice("no-deploy") {
		if !strings.HasSuffix(noDeploy, ".yaml") {
			noDeploy = noDeploy + ".yaml"
		}
		serverConfig.ControlConfig.Skips = append(serverConfig.ControlConfig.Skips, noDeploy)
	}

	logrus.Info("Starting k3s ", app.App.Version)
	ctx := signal.SigTermCancelContext(context.Background())
	certs, err := server.StartServer(ctx, &serverConfig)
	if err != nil {
		return err
	}

	if cfg.DisableAgent {
		<-ctx.Done()
		return nil
	}

	logFile := filepath.Join(serverConfig.ControlConfig.DataDir, "agent/agent.log")
	url := fmt.Sprintf("https://localhost:%d", serverConfig.TLSConfig.HTTPSPort)
	logrus.Infof("Agent starting, logging to %s", logFile)
	token := server.FormatToken(serverConfig.ControlConfig.Runtime.NodeToken, certs)

	agentConfig := cmds.AgentConfig
	agentConfig.DataDir = filepath.Dir(serverConfig.ControlConfig.DataDir)
	agentConfig.ServerURL = url
	agentConfig.Token = token

	return agent.Run(ctx, agentConfig)
}
