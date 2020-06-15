package containerd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/natefinch/lumberjack"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent/templates"
	util2 "github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	yaml "gopkg.in/yaml.v2"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
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
		cmd.Env = os.Environ()
		// elide NOTIFY_SOCKET to prevent spurious notifications to systemd
		for i := range cmd.Env {
			if strings.HasPrefix(cmd.Env[i], "NOTIFY_SOCKET=") {
				cmd.Env = append(cmd.Env[:i], cmd.Env[i+1:]...)
				break
			}
		}
		addDeathSig(cmd)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "containerd: %s\n", err)
		}
		os.Exit(1)
	}()

	first := true
	for {
		conn, err := criConnection(ctx, cfg.Containerd.Address)
		if err == nil {
			conn.Close()
			break
		}
		if first {
			first = false
		} else {
			logrus.Infof("Waiting for containerd startup: %v", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return preloadImages(ctx, cfg)
}

func criConnection(ctx context.Context, address string) (*grpc.ClientConn, error) {
	addr, dialer, err := util.GetAddressAndDialer("unix://" + address)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithTimeout(3*time.Second), grpc.WithContextDialer(dialer), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
	if err != nil {
		return nil, err
	}

	c := runtimeapi.NewRuntimeServiceClient(conn)
	_, err = c.Version(ctx, &runtimeapi.VersionRequest{
		Version: "0.1.0",
	})
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func preloadImages(ctx context.Context, cfg *config.Node) error {
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

	criConn, err := criConnection(ctx, cfg.Containerd.Address)
	if err != nil {
		return err
	}
	defer criConn.Close()

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

		if strings.HasSuffix(fileInfo.Name(), ".txt") {
			prePullImages(ctx, criConn, file)
			file.Close()
			continue
		}

		logrus.Debugf("Import %s", filePath)
		_, err = client.Import(ctxContainerD, file)
		file.Close()
		if err != nil {
			logrus.Errorf("Unable to import %s: %v", filePath, err)
		}
	}
	return nil
}

func prePullImages(ctx context.Context, conn *grpc.ClientConn, images io.Reader) {
	imageClient := runtimeapi.NewImageServiceClient(conn)
	scanner := bufio.NewScanner(images)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		resp, err := imageClient.ImageStatus(ctx, &runtimeapi.ImageStatusRequest{
			Image: &runtimeapi.ImageSpec{
				Image: line,
			},
		})
		if err == nil && resp.Image != nil {
			continue
		}

		logrus.Infof("Pulling image %s...", line)
		_, err = imageClient.PullImage(ctx, &runtimeapi.PullImageRequest{
			Image: &runtimeapi.ImageSpec{
				Image: line,
			},
		})
		if err != nil {
			logrus.Errorf("Failed to pull %s: %v", line, err)
		}
	}
}

func setupContainerdConfig(ctx context.Context, cfg *config.Node) error {
	privRegistries, err := getPrivateRegistries(ctx, cfg)
	if err != nil {
		return err
	}
	var containerdTemplate string
	containerdConfig := templates.ContainerdConfig{
		NodeConfig:            cfg,
		IsRunningInUserNS:     system.RunningInUserNS(),
		PrivateRegistryConfig: privRegistries,
	}

	selEnabled, selConfigured, err := selinuxStatus()
	if err != nil {
		return errors.Wrap(err, "failed to detect selinux")
	}
	if cfg.DisableSELinux {
		containerdConfig.SELinuxEnabled = false
		if selEnabled {
			logrus.Warn("SELinux is enabled for system but has been disabled for containerd by override")
		}
	} else {
		containerdConfig.SELinuxEnabled = selEnabled
	}
	if containerdConfig.SELinuxEnabled && !selConfigured {
		logrus.Warnf("SELinux is enabled for "+version.Program+" but process is not running in context '%s', "+version.Program+"-selinux policy may need to be applied", SELinuxContextType)
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

func getPrivateRegistries(ctx context.Context, cfg *config.Node) (*templates.Registry, error) {
	privRegistries := &templates.Registry{}
	privRegistryFile, err := ioutil.ReadFile(cfg.AgentConfig.PrivateRegistry)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	logrus.Infof("Using registry config file at %s", cfg.AgentConfig.PrivateRegistry)
	if err := yaml.Unmarshal(privRegistryFile, &privRegistries); err != nil {
		return nil, err
	}
	return privRegistries, nil
}
