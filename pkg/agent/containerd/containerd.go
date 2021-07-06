package containerd

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
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
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/reference/docker"
	"github.com/klauspost/compress/zstd"
	"github.com/natefinch/lumberjack"
	"github.com/pierrec/lz4"
	"github.com/pkg/errors"
	util2 "github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/untar"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

const (
	maxMsgSize = 1024 * 1024 * 16
)

// Run configures and starts containerd as a child process. Once it is up, images are preloaded
// or pulled from files found in the agent images directory.
func Run(ctx context.Context, cfg *config.Node) error {
	args := getContainerdArgs(cfg)

	if err := setupContainerdConfig(ctx, cfg); err != nil {
		return err
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
		env := []string{}

		for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			switch {
			case pair[0] == "NOTIFY_SOCKET":
				// elide NOTIFY_SOCKET to prevent spurious notifications to systemd
			case pair[0] == "CONTAINERD_LOG_LEVEL":
				// Turn CONTAINERD_LOG_LEVEL variable into log-level flag
				args = append(args, "--log-level", pair[1])
			case strings.HasPrefix(pair[0], "CONTAINERD_"):
				// Strip variables with CONTAINERD_ prefix before passing through
				// This allows doing things like setting a proxy for image pulls by setting
				// CONTAINERD_https_proxy=http://proxy.example.com:8080
				pair[0] = strings.TrimPrefix(pair[0], "CONTAINERD_")
				fallthrough
			default:
				env = append(env, strings.Join(pair, "="))
			}
		}

		logrus.Infof("Running containerd %s", config.ArgString(args[1:]))
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout = stdOut
		cmd.Stderr = stdErr
		cmd.Env = env

		addDeathSig(cmd)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "containerd: %s\n", err)
		}
		os.Exit(1)
	}()

	first := true
	for {
		conn, err := CriConnection(ctx, cfg.Containerd.Address)
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
	logrus.Info("Containerd is now running")

	return preloadImages(ctx, cfg)
}

// preloadImages reads the contents of the agent images directory, and attempts to
// import into containerd any files found there. Supported compressed types are decompressed, and
// any .txt files are processed as a list of images that should be pre-pulled from remote registries.
// If configured, imported images are retagged as being pulled from additional registries.
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

	criConn, err := CriConnection(ctx, cfg.Containerd.Address)
	if err != nil {
		return err
	}
	defer criConn.Close()

	// Ensure that nothing else can modify the image store while we're importing,
	// and that our images are imported into the k8s.io namespace
	ctx = namespaces.WithNamespace(ctx, "k8s.io")
	// At startup all leases from k3s are cleared
	ls := client.LeasesService()
	existingLeases, err := ls.List(ctx)
	if err != nil {
		return err
	}

	for _, lease := range existingLeases {
		if lease.ID == version.Program {
			logrus.Debugf("Deleting existing lease: %v", lease)
			ls.Delete(ctx, lease)
		}
	}

	// Any images found on import are given a lease that never expires
	_, err = ls.Create(ctx, leases.WithID(version.Program))
	if err != nil {
		return err
	}

	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		start := time.Now()
		filePath := filepath.Join(cfg.Images, fileInfo.Name())

		if err := preloadFile(ctx, cfg, client, criConn, filePath); err != nil {
			logrus.Errorf("Error encountered while importing %s: %v", filePath, err)
			continue
		}
		logrus.Debugf("Imported images from %s in %s", filePath, time.Since(start))
	}
	return nil
}

// preloadFile handles loading images from a single tarball or pre-pull image list.
// This is in its own function so that we can ensure that the various readers are properly closed, as some
// decompressing readers need to be explicitly closed and others do not.
func preloadFile(ctx context.Context, cfg *config.Node, client *containerd.Client, criConn *grpc.ClientConn, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var imageReader io.Reader
	switch {
	case util2.HasSuffixI(filePath, ".txt"):
		return prePullImages(ctx, criConn, file)
	case util2.HasSuffixI(filePath, ".tar"):
		imageReader = file
	case util2.HasSuffixI(filePath, ".tar.lz4"):
		imageReader = lz4.NewReader(file)
	case util2.HasSuffixI(filePath, ".tar.bz2", ".tbz"):
		imageReader = bzip2.NewReader(file)
	case util2.HasSuffixI(filePath, ".tar.gz", ".tgz"):
		zr, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer zr.Close()
		imageReader = zr
	case util2.HasSuffixI(filePath, "tar.zst", ".tzst"):
		zr, err := zstd.NewReader(file, zstd.WithDecoderMaxMemory(untar.MaxDecoderMemory))
		if err != nil {
			return err
		}
		defer zr.Close()
		imageReader = zr
	default:
		return errors.New("unhandled file type")
	}

	logrus.Infof("Importing images from %s", filePath)

	images, err := client.Import(ctx, imageReader, containerd.WithAllPlatforms(true))
	if err != nil {
		return err
	}

	return retagImages(ctx, client, images, cfg.AgentConfig.AirgapExtraRegistry)
}

// retagImages retags all listed images as having been pulled from the given remote registries.
// If duplicate images exist, they are overwritten. This is most useful when using a private registry
// for all images, as can be configured by the RKE2/Rancher system-default-registry setting.
func retagImages(ctx context.Context, client *containerd.Client, images []images.Image, registries []string) error {
	var errs []error
	imageService := client.ImageService()
	for _, image := range images {
		name, err := parseNamedTagged(image.Name)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to parse image name"))
			continue
		}
		logrus.Infof("Imported %s", image.Name)
		for _, registry := range registries {
			image.Name = fmt.Sprintf("%s/%s:%s", registry, docker.Path(name), name.Tag())
			if _, err = imageService.Create(ctx, image); err != nil {
				if errdefs.IsAlreadyExists(err) {
					if err = imageService.Delete(ctx, image.Name); err != nil {
						errs = append(errs, errors.Wrap(err, "failed to delete existing image"))
						continue
					}
					if _, err = imageService.Create(ctx, image); err != nil {
						errs = append(errs, errors.Wrap(err, "failed to tag after deleting existing image"))
						continue
					}
				} else {
					errs = append(errs, errors.Wrap(err, "failed to tag image"))
					continue
				}
			}
			logrus.Infof("Tagged %s", image.Name)
		}
	}
	return merr.NewErrors(errs...)
}

// parseNamedTagged parses and normalizes an image name, and converts the resulting reference
// to a type that exposes the tag.
func parseNamedTagged(name string) (docker.NamedTagged, error) {
	ref, err := docker.ParseNormalizedNamed(name)
	if err != nil {
		return nil, err
	}
	tagged, ok := ref.(docker.NamedTagged)
	if !ok {
		return nil, fmt.Errorf("can't cast %T to NamedTagged", ref)
	}
	return tagged, nil
}

// prePullImages asks containerd to pull images in a given list, so that they
// are ready when the containers attempt to start later.
func prePullImages(ctx context.Context, conn *grpc.ClientConn, images io.Reader) error {
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
	return nil
}
