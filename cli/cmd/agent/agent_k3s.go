// +build k8s

package agent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/rancher/norman/pkg/clientaccess"
	"github.com/rancher/norman/pkg/resolvehome"
	"github.com/rancher/rio/pkg/enterchroot"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func (a *Agent) Run(ctx *cli.Context) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("agent must be ran as root")
	}

	if len(a.T_Token) == 0 {
		return fmt.Errorf("--token is required")
	}

	if len(a.S_Server) == 0 {
		return fmt.Errorf("--server is required")
	}

	dataDir, err := resolvehome.Resolve(a.D_DataDir)
	if err != nil {
		return err
	}

	return RunAgent(a.S_Server, a.T_Token, dataDir, a.L_Log, a.I_NodeIp)
}

func RunAgent(server, token, dataDir, logFile, ipAddress string) error {
	dataDir = filepath.Join(dataDir, "agent")

	for {
		tmpFile, err := clientaccess.AgentAccessInfoToTempKubeConfig("", server, token)
		if err != nil {
			logrus.Error(err)
			time.Sleep(2 * time.Second)
			continue
		}
		os.Remove(tmpFile)
		break
	}

	os.Setenv("K3S_URL", server)
	os.Setenv("K3S_TOKEN", token)
	os.Setenv("K3S_DATA_DIR", dataDir)
	os.Setenv("K3S_NODE_IP", ipAddress)

	os.MkdirAll(dataDir, 0700)

	stdout := io.Writer(os.Stdout)
	stderr := io.Writer(os.Stderr)

	if logFile == "" {
		stdout = os.Stdout
		stderr = os.Stderr
	} else {
		l := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    50,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		}
		stdout = l
		stderr = l
	}

	return enterchroot.Mount(filepath.Join(dataDir, "root"), stdout, stderr, os.Args[1:])
}
