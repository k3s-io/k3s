package containerd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/natefinch/lumberjack"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/rancher/k3s/pkg/agent/templates"
	util2 "github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	maxMsgSize = 1024 * 1024 * 16
)

func Run(ctx context.Context, cfg *config.Node) error {
	args := []string{
		"containerd",
		"-c", cfg.Containerd.Config,
		"-a", cfg.Containerd.Address,
		"--state", cfg.Containerd.State,
		"--root", cfg.Containerd.Root,
	}

	if err := setupContainerdConfig(ctx, cfg); err != nil {
		return err
	}

	if os.Getenv("CONTAINERD_LOG_LEVEL") != "" {
		args = append(args, "-l", os.Getenv("CONTAINERD_LOG_LEVEL"))
	}

	stdOut := io.Writer(os.Stdout)
	stdErr := io.Writer(os.Stderr)

	if cfg.Containerd.Log != "" {
		logrus.Infof("Logging containerd to %s", cfg.Containerd.Log)
		stdOut = &lumberjack.Logger{
			Filename:   cfg.Containerd.Log,
			MaxSize:    50,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		}
		stdErr = stdOut
	}

	go func() {
		logrus.Infof("Running containerd %s", config.ArgString(args[1:]))
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = stdOut
		cmd.Stderr = stdErr
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

	return preloadImages(cfg)
}

func preloadImages(cfg *config.Node) error {
	fileInfo, err := os.Stat(cfg.Images)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		logrus.Errorf("Unable to find images in %s: %v", cfg.Images, err)
		return nil
	}

	if !fileInfo.IsDir() {
		return nil
	}

	fileInfos, err := ioutil.ReadDir(cfg.Images)
	if err != nil {
		logrus.Errorf("Unable to read images in %s: %v", cfg.Images, err)
		return nil
	}

	client, err := containerd.New(cfg.Containerd.Address)
	if err != nil {
		return err
	}
	defer client.Close()

	ctxContainerD := namespaces.WithNamespace(context.Background(), "k8s.io")

	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		filePath := filepath.Join(cfg.Images, fileInfo.Name())

		file, err := os.Open(filePath)
		if err != nil {
			logrus.Errorf("Unable to read %s: %v", filePath, err)
			continue
		}

		logrus.Debugf("Import %s", filePath)
		_, err = client.Import(ctxContainerD, file)
		if err != nil {
			logrus.Errorf("Unable to import %s: %v", filePath, err)
		}
	}
	return nil
}

func setupContainerdConfig(ctx context.Context, cfg *config.Node) error {
	var containerdTemplate string
	containerdConfig := templates.ContainerdConfig{
		NodeConfig:        cfg,
		IsRunningInUserNS: system.RunningInUserNS(),
	}

	containerdTemplateBytes, err := ioutil.ReadFile(cfg.Containerd.Template)
	if err == nil {
		logrus.Infof("Using containerd template at %s", cfg.Containerd.Template)
		containerdTemplate = string(containerdTemplateBytes)
	} else if os.IsNotExist(err) {
		containerdTemplate = templates.ContainerdConfigTemplate
	} else {
		return err
	}
	parsedTemplate, err := templates.ParseTemplateFromConfig(containerdTemplate, containerdConfig)
	if err != nil {
		return err
	}

	return util2.WriteFile(cfg.Containerd.Config, parsedTemplate)
}
