package containerd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	util2 "github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	maxMsgSize = 1024 * 1024 * 16
	configToml = `
[plugins.opt]
	path = "%OPT%"
[plugins.cri]
  stream_server_address = "%NODE%"
  stream_server_port = "10010"
`
	configCNIToml = `
  [plugins.cri.cni]
    bin_dir = "%CNIBIN%"
    conf_dir = "%CNICFG%"
`
)

func Run(ctx context.Context, cfg *config.Node) error {
	args := []string{
		"containerd",
		"-c", cfg.Containerd.Config,
		"-a", cfg.Containerd.Address,
		"--state", cfg.Containerd.State,
		"--root", cfg.Containerd.Root,
	}

	template := configToml
	if !cfg.NoFlannel {
		template += configCNIToml
	}

	template = strings.Replace(template, "%OPT%", cfg.Containerd.Opt, -1)
	template = strings.Replace(template, "%CNIBIN%", cfg.AgentConfig.CNIBinDir, -1)
	template = strings.Replace(template, "%CNICFG%", cfg.AgentConfig.CNIConfDir, -1)
	template = strings.Replace(template, "%NODE%", cfg.AgentConfig.NodeName, -1)

	if err := util2.WriteFile(cfg.Containerd.Config, template); err != nil {
		return err
	}

	if logrus.GetLevel() >= logrus.DebugLevel {
		args = append(args, "-l", "debug")
	}

	go func() {
		logrus.Infof("Running containerd %s", config.ArgString(args[1:]))
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGKILL,
		}
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "containerd: %s\n", err)
		}
		os.Exit(1)
	}()

	for {
		addr, dailer, err := util.GetAddressAndDialer("unix://" + cfg.Containerd.Address)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(3*time.Second), grpc.WithDialer(dailer), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		c := runtimeapi.NewRuntimeServiceClient(conn)

		_, err = c.Version(ctx, &runtimeapi.VersionRequest{
			Version: "0.1.0",
		})
		if err == nil {
			conn.Close()
			break
		}
		conn.Close()
		logrus.Infof("Waiting for containerd startup: %v", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return nil
}
