package containerd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/rancher/rio/agent/config"
	util2 "github.com/rancher/rio/agent/util"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	address    = "/run/k3s/containerd.sock"
	maxMsgSize = 1024 * 1024 * 16
	configToml = `[plugins.cri]
  stream_server_address = "%NODE%"
  stream_server_port = "10010"
  [plugins.cri.cni]
    bin_dir = "/usr/share/cni/bin"
    conf_dir = "/etc/cni/net.d"
`
)

func Run(ctx context.Context, config *config.NodeConfig) error {
	args := []string{
		"containerd",
		"-a", address,
		"--state", "/run/k3s/containerd",
	}

	if err := util2.WriteFile("/etc/containerd/config.toml",
		strings.Replace(configToml, "%NODE%", config.AgentConfig.NodeName, -1)); err != nil {
		return err
	}

	if logrus.GetLevel() >= logrus.DebugLevel {
		args = append(args, "--verbose")
	}

	go func() {
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
		addr, dailer, err := util.GetAddressAndDialer("unix://" + address)
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
		logrus.Infof("Waiting for containerd startup")
		time.Sleep(1 * time.Second)
	}

	return nil
}
