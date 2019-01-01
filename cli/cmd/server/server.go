package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/pkg/reexec"
	"github.com/natefinch/lumberjack"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rio/pkg/server"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/util/net"
)

var (
	appName = filepath.Base(os.Args[0])

	config server.Config
	log    string
)

var ServerCommand = cli.Command{
	Name:      "server",
	Usage:     "Run management server",
	UsageText: appName + " server [OPTIONS]",
	Action:    Run,
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:        "https-listen-port",
			Usage:       "HTTPS listen port",
			Value:       6443,
			Destination: &config.TLSConfig.HTTPSPort,
		},
		cli.IntFlag{
			Name:        "http-listen-port",
			Usage:       "HTTP listen port (for /healthz, HTTPS redirect, and port for TLS terminating LB)",
			Value:       0,
			Destination: &config.TLSConfig.HTTPPort,
		},
		cli.StringFlag{
			Name:        "data-dir",
			Usage:       "Folder to hold state default /var/lib/rancher/k3s or ${HOME}/.rancher/k3s if not root",
			Destination: &config.ControlConfig.DataDir,
		},
		//cli.StringFlag{
		//	Name:        "advertise-address",
		//	Usage:       "Address of the server to put in the generated kubeconfig",
		//	Destination: &config.AdvertiseIP,
		//},
		cli.BoolFlag{
			Name:        "disable-agent",
			Usage:       "Do not run a local agent and register a local kubelet",
			Destination: &config.DisableAgent,
		},
		cli.StringFlag{
			Name:        "log",
			Usage:       "Log to file",
			Destination: &log,
		},
	},
}

func setupLogging(app *cli.Context) {
	if !app.GlobalBool("debug") {
		flag.Set("stderrthreshold", "3")
		flag.Set("alsologtostderr", "false")
		flag.Set("logtostderr", "false")
	}
}

func runWithLogging(app *cli.Context) error {
	l := &lumberjack.Logger{
		Filename:   log,
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
	if log != "" && os.Getenv("_RIO_REEXEC_") == "" {
		return runWithLogging(app)
	}

	setupLogging(app)

	if !config.DisableAgent && os.Getuid() != 0 {
		return fmt.Errorf("must run as root unless --disable-agent is specified")
	}

	if config.ControlConfig.NodeConfig.AgentConfig.NodeIP == "" {
		ip, err := net.ChooseHostInterface()
		if err == nil {
			config.ControlConfig.NodeConfig.AgentConfig.NodeIP = ip.String()
		}
	}

	logrus.Info("Starting k3s ", app.App.Version)
	ctx := signal.SigTermCancelContext(context.Background())
	if err := server.StartServer(ctx, &config); err != nil {
		return err
	}

	if config.DisableAgent {
		<-ctx.Done()
		return nil
	}

	return nil
	//logFile := filepath.Join(serverConfig.DataDir, "agent/agent.log")
	//url := fmt.Sprintf("https://localhost:%d", httpsListenPort)
	//logrus.Infof("Agent starting, logging to %s", logFile)
	//return agent.RunAgent(url, server2.FormatToken(serverConfig.Runtime.NodeToken), serverConfig.DataDir, logFile, "")
}
