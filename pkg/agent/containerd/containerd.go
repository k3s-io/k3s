package containerd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/errdefs"
	docker "github.com/distribution/reference"
	reference "github.com/google/go-containerregistry/pkg/name"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	util2 "github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/natefinch/lumberjack"
	"github.com/pkg/errors"
	"github.com/rancher/wharfie/pkg/tarfile"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

var (
	// In addition to using the CRI pinned label, we add our own label to indicate that
	// the image was pinned by the import process, so that we can clear the pin on subsequent startups.
	// ref: https://github.com/containerd/containerd/blob/release/1.7/pkg/cri/labels/labels.go
	k3sPinnedImageLabelKey   = "io.cattle." + version.Program + ".pinned"
	k3sPinnedImageLabelValue = "pinned"
)

const (
	// these were previously exported via containerd/containerd/pkg/cri/constants
	// and containerd/containerd/pkg/cri/labels but have been made internal as of
	// containerd v2.
	criContainerdPrefix       = "io.cri-containerd"
	criPinnedImageLabelKey    = criContainerdPrefix + ".pinned"
	criPinnedImageLabelValue  = "pinned"
	criK8sContainerdNamespace = "k8s.io"
)

// Run configures and starts containerd as a child process. Once it is up, images are preloaded
// or pulled from files found in the agent images directory.
func Run(ctx context.Context, cfg *config.Node) error {
	args := getContainerdArgs(cfg)
	stdOut := io.Writer(os.Stdout)
	stdErr := io.Writer(os.Stderr)

	if cfg.Containerd.Log != "" {
		logrus.Infof("Logging containerd to %s", cfg.Containerd.Log)
		fileOut := &lumberjack.Logger{
			Filename:   cfg.Containerd.Log,
			MaxSize:    50,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		}
		// If k3s is started with --debug, write logs to both the log file and stdout/stderr,
		// even if a log path is set.
		if cfg.Containerd.Debug {
			stdOut = io.MultiWriter(stdOut, fileOut)
			stdErr = io.MultiWriter(stdErr, fileOut)
		} else {
			stdOut = fileOut
			stdErr = fileOut
		}
	}

	go func() {
		env := []string{}
		cenv := []string{}

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
				cenv = append(cenv, strings.Join(pair, "="))
			default:
				env = append(env, strings.Join(pair, "="))
			}
		}

		logrus.Infof("Running containerd %s", config.ArgString(args[1:]))
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = stdOut
		cmd.Stderr = stdErr
		cmd.Env = append(env, cenv...)

		addDeathSig(cmd)
		err := cmd.Run()
		if err != nil && !errors.Is(err, context.Canceled) {
			logrus.Errorf("containerd exited: %s", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	if err := cri.WaitForService(ctx, cfg.Containerd.Address, "containerd"); err != nil {
		return err
	}

	return PreloadImages(ctx, cfg)
}

// PreloadImages reads the contents of the agent images directory, and attempts to
// import into containerd any files found there. Supported compressed types are decompressed, and
// any .txt files are processed as a list of images that should be pre-pulled from remote registries.
// If configured, imported images are retagged as being pulled from additional registries.
func PreloadImages(ctx context.Context, cfg *config.Node) error {
	client, err := Client(cfg.Containerd.Address)
	if err != nil {
		return err
	}
	defer client.Close()

	// Image pulls must be done using the CRI client, not the containerd client.
	// Repository mirrors and rewrites are handled by the CRI service; if you pull directly
	// using the containerd image service it will ignore the configured settings.
	criConn, err := cri.Connection(ctx, cfg.Containerd.Address)
	if err != nil {
		return err
	}
	defer criConn.Close()
	imageClient := runtimeapi.NewImageServiceClient(criConn)

	// Ensure that our images are imported into the correct namespace
	ctx = namespaces.WithNamespace(ctx, criK8sContainerdNamespace)

	// At startup all leases from k3s are cleared; we no longer use leases to lock content
	if err := clearLeases(ctx, client); err != nil {
		return errors.Wrap(err, "failed to clear leases")
	}

	// Clear the pinned labels on all images previously pinned by k3s
	if err := clearLabels(ctx, client); err != nil {
		return errors.Wrap(err, "failed to clear pinned labels")
	}

	go watchImages(ctx, cfg)

	// After setting the watcher, connections and everything, k3s will see if the images folder is already created
	// if the folder its already created, it will load the images
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

	fileInfos, err := os.ReadDir(cfg.Images)
	if err != nil {
		logrus.Errorf("Unable to read images in %s: %v", cfg.Images, err)
		return nil
	}

	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}

		start := time.Now()
		filePath := filepath.Join(cfg.Images, fileInfo.Name())

		if err := preloadFile(ctx, cfg, client, imageClient, filePath); err != nil {
			logrus.Errorf("Error encountered while importing %s: %v", filePath, err)
			continue
		}
		logrus.Infof("Imported images from %s in %s", filePath, time.Since(start))
	}

	return nil
}

// preloadFile handles loading images from a single tarball or pre-pull image list.
// This is in its own function so that we can ensure that the various readers are properly closed, as some
// decompressing readers need to be explicitly closed and others do not.
func preloadFile(ctx context.Context, cfg *config.Node, client *containerd.Client, imageClient runtimeapi.ImageServiceClient, filePath string) error {
	var images []images.Image
	if util2.HasSuffixI(filePath, ".txt") {
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()
		logrus.Infof("Pulling images from %s", filePath)
		images, err = prePullImages(ctx, client, imageClient, file)
		if err != nil {
			return errors.Wrap(err, "failed to pull images from "+filePath)
		}
	} else {
		opener, err := tarfile.GetOpener(filePath)
		if err != nil {
			return err
		}

		imageReader, err := opener()
		if err != nil {
			return err
		}
		defer imageReader.Close()

		logrus.Infof("Importing images from %s", filePath)
		images, err = client.Import(ctx, imageReader, containerd.WithAllPlatforms(true), containerd.WithSkipMissing())
		if err != nil {
			return errors.Wrap(err, "failed to import images from "+filePath)
		}
	}

	if err := labelImages(ctx, client, images, filepath.Base(filePath)); err != nil {
		return errors.Wrap(err, "failed to add pinned label to images")
	}
	if err := retagImages(ctx, client, images, cfg.AgentConfig.AirgapExtraRegistry); err != nil {
		return errors.Wrap(err, "failed to retag images")
	}

	for _, image := range images {
		logrus.Infof("Imported %s", image.Name)
	}
	return nil
}

// clearLeases deletes any leases left by previous versions of k3s.
// We no longer use leases to lock content; they only locked the
// blobs, not the actual images.
func clearLeases(ctx context.Context, client *containerd.Client) error {
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
	return nil
}

// clearLabels removes the pinned labels on all images in the image store that were previously pinned by k3s
func clearLabels(ctx context.Context, client *containerd.Client) error {
	var errs []error
	imageService := client.ImageService()
	images, err := imageService.List(ctx, fmt.Sprintf("labels.%q==%s", k3sPinnedImageLabelKey, k3sPinnedImageLabelValue))
	if err != nil {
		return err
	}
	for _, image := range images {
		delete(image.Labels, k3sPinnedImageLabelKey)
		delete(image.Labels, criPinnedImageLabelKey)
		if _, err := imageService.Update(ctx, image, "labels"); err != nil {
			errs = append(errs, errors.Wrap(err, "failed to delete labels from image "+image.Name))
		}
	}
	return merr.NewErrors(errs...)
}

// labelImages adds labels to the listed images, indicating that they
// are pinned by k3s and should not be pruned.
func labelImages(ctx context.Context, client *containerd.Client, images []images.Image, fileName string) error {
	var errs []error
	imageService := client.ImageService()
	for i, image := range images {
		if image.Labels[k3sPinnedImageLabelKey] == k3sPinnedImageLabelValue &&
			image.Labels[criPinnedImageLabelKey] == criPinnedImageLabelValue {
			continue
		}

		if image.Labels == nil {
			image.Labels = map[string]string{}
		}

		image.Labels[k3sPinnedImageLabelKey] = k3sPinnedImageLabelValue
		image.Labels[criPinnedImageLabelKey] = criPinnedImageLabelValue
		updatedImage, err := imageService.Update(ctx, image, "labels")
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to add labels to image "+image.Name))
		} else {
			images[i] = updatedImage
		}
	}
	return merr.NewErrors(errs...)
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
			errs = append(errs, errors.Wrap(err, "failed to parse tags for image "+image.Name))
			continue
		}
		for _, registry := range registries {
			newName := fmt.Sprintf("%s/%s:%s", registry, docker.Path(name), name.Tag())
			if newName == image.Name {
				continue
			}
			image.Name = newName
			if _, err = imageService.Create(ctx, image); err != nil {
				if errdefs.IsAlreadyExists(err) {
					if err = imageService.Delete(ctx, image.Name); err != nil {
						errs = append(errs, errors.Wrap(err, "failed to delete existing image "+image.Name))
						continue
					}
					if _, err = imageService.Create(ctx, image); err != nil {
						errs = append(errs, errors.Wrap(err, "failed to tag after deleting existing image "+image.Name))
						continue
					}
				} else {
					errs = append(errs, errors.Wrap(err, "failed to tag image "+image.Name))
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
// are ready when the containers attempt to start later. If the image already exists,
// or is successfully pulled, information about the image is retrieved from the image store.
// NOTE: Pulls MUST be done via CRI API, not containerd API, in order to use mirrors and rewrites.
func prePullImages(ctx context.Context, client *containerd.Client, imageClient runtimeapi.ImageServiceClient, imageList io.Reader) ([]images.Image, error) {
	errs := []error{}
	images := []images.Image{}
	imageService := client.ImageService()
	scanner := bufio.NewScanner(imageList)
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())

		if name == "" {
			continue
		}

		// the options in the reference.ParseReference are for filtering only strings that cannot be seen as a possible image
		if _, err := reference.ParseReference(name, reference.WeakValidation, reference.Insecure); err != nil {
			logrus.Errorf("Failed to parse image reference %q: %v", name, err)
			continue
		}

		if status, err := imageClient.ImageStatus(ctx, &runtimeapi.ImageStatusRequest{
			Image: &runtimeapi.ImageSpec{
				Image: name,
			},
		}); err == nil && status.Image != nil && len(status.Image.RepoTags) > 0 {
			logrus.Infof("Image %s has already been pulled", name)
			for _, tag := range status.Image.RepoTags {
				if image, err := imageService.Get(ctx, tag); err != nil {
					errs = append(errs, err)
				} else {
					images = append(images, image)
				}
			}
			continue
		}

		logrus.Infof("Pulling image %s", name)
		if _, err := imageClient.PullImage(ctx, &runtimeapi.PullImageRequest{
			Image: &runtimeapi.ImageSpec{
				Image: name,
			},
		}); err != nil {
			errs = append(errs, err)
		} else {
			if image, err := imageService.Get(ctx, name); err != nil {
				errs = append(errs, err)
			} else {
				images = append(images, image)
			}
		}
	}
	return images, merr.NewErrors(errs...)
}
