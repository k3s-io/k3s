/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package server

import (
	"path/filepath"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/snapshots"
	"github.com/containerd/typeurl"
	"github.com/davecgh/go-spew/spew"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	cio "github.com/containerd/containerd/pkg/cri/io"
	customopts "github.com/containerd/containerd/pkg/cri/opts"
	containerstore "github.com/containerd/containerd/pkg/cri/store/container"
	"github.com/containerd/containerd/pkg/cri/util"
	ctrdutil "github.com/containerd/containerd/pkg/cri/util"
)

func init() {
	typeurl.Register(&containerstore.Metadata{},
		"github.com/containerd/cri/pkg/store/container", "Metadata")
}

// CreateContainer creates a new container in the given PodSandbox.
func (c *criService) CreateContainer(ctx context.Context, r *runtime.CreateContainerRequest) (_ *runtime.CreateContainerResponse, retErr error) {
	config := r.GetConfig()
	log.G(ctx).Debugf("Container config %+v", config)
	sandboxConfig := r.GetSandboxConfig()
	sandbox, err := c.sandboxStore.Get(r.GetPodSandboxId())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find sandbox id %q", r.GetPodSandboxId())
	}
	sandboxID := sandbox.ID
	s, err := sandbox.Container.Task(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sandbox container task")
	}
	sandboxPid := s.Pid()

	// Generate unique id and name for the container and reserve the name.
	// Reserve the container name to avoid concurrent `CreateContainer` request creating
	// the same container.
	id := util.GenerateID()
	metadata := config.GetMetadata()
	if metadata == nil {
		return nil, errors.New("container config must include metadata")
	}
	containerName := metadata.Name
	name := makeContainerName(metadata, sandboxConfig.GetMetadata())
	log.G(ctx).Debugf("Generated id %q for container %q", id, name)
	if err = c.containerNameIndex.Reserve(name, id); err != nil {
		return nil, errors.Wrapf(err, "failed to reserve container name %q", name)
	}
	defer func() {
		// Release the name if the function returns with an error.
		if retErr != nil {
			c.containerNameIndex.ReleaseByName(name)
		}
	}()

	// Create initial internal container metadata.
	meta := containerstore.Metadata{
		ID:        id,
		Name:      name,
		SandboxID: sandboxID,
		Config:    config,
	}

	// Prepare container image snapshot. For container, the image should have
	// been pulled before creating the container, so do not ensure the image.
	image, err := c.localResolve(config.GetImage().GetImage())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve image %q", config.GetImage().GetImage())
	}
	containerdImage, err := c.toContainerdImage(ctx, image)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get image from containerd %q", image.ID)
	}

	// Run container using the same runtime with sandbox.
	sandboxInfo, err := sandbox.Container.Info(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get sandbox %q info", sandboxID)
	}

	// Create container root directory.
	containerRootDir := c.getContainerRootDir(id)
	if err = c.os.MkdirAll(containerRootDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create container root directory %q",
			containerRootDir)
	}
	defer func() {
		if retErr != nil {
			// Cleanup the container root directory.
			if err = c.os.RemoveAll(containerRootDir); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to remove container root directory %q",
					containerRootDir)
			}
		}
	}()
	volatileContainerRootDir := c.getVolatileContainerRootDir(id)
	if err = c.os.MkdirAll(volatileContainerRootDir, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create volatile container root directory %q",
			volatileContainerRootDir)
	}
	defer func() {
		if retErr != nil {
			// Cleanup the volatile container root directory.
			if err = c.os.RemoveAll(volatileContainerRootDir); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to remove volatile container root directory %q",
					volatileContainerRootDir)
			}
		}
	}()

	var volumeMounts []*runtime.Mount
	if !c.config.IgnoreImageDefinedVolumes {
		// Create container image volumes mounts.
		volumeMounts = c.volumeMounts(containerRootDir, config.GetMounts(), &image.ImageSpec.Config)
	} else if len(image.ImageSpec.Config.Volumes) != 0 {
		log.G(ctx).Debugf("Ignoring volumes defined in image %v because IgnoreImageDefinedVolumes is set", image.ID)
	}

	// Generate container mounts.
	mounts := c.containerMounts(sandboxID, config)

	ociRuntime, err := c.getSandboxRuntime(sandboxConfig, sandbox.Metadata.RuntimeHandler)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sandbox runtime")
	}
	log.G(ctx).Debugf("Use OCI runtime %+v for sandbox %q and container %q", ociRuntime, sandboxID, id)

	spec, err := c.containerSpec(id, sandboxID, sandboxPid, sandbox.NetNSPath, containerName, containerdImage.Name(), config, sandboxConfig,
		&image.ImageSpec.Config, append(mounts, volumeMounts...), ociRuntime)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate container %q spec", id)
	}

	meta.ProcessLabel = spec.Process.SelinuxLabel

	// handle any KVM based runtime
	if err := modifyProcessLabel(ociRuntime.Type, spec); err != nil {
		return nil, err
	}

	if config.GetLinux().GetSecurityContext().GetPrivileged() {
		// If privileged don't set the SELinux label but still record it on the container so
		// the unused MCS label can be release later
		spec.Process.SelinuxLabel = ""
	}
	defer func() {
		if retErr != nil {
			selinux.ReleaseLabel(spec.Process.SelinuxLabel)
		}
	}()

	log.G(ctx).Debugf("Container %q spec: %#+v", id, spew.NewFormatter(spec))

	snapshotterOpt := snapshots.WithLabels(snapshots.FilterInheritedLabels(config.Annotations))
	// Set snapshotter before any other options.
	opts := []containerd.NewContainerOpts{
		containerd.WithSnapshotter(c.config.ContainerdConfig.Snapshotter),
		// Prepare container rootfs. This is always writeable even if
		// the container wants a readonly rootfs since we want to give
		// the runtime (runc) a chance to modify (e.g. to create mount
		// points corresponding to spec.Mounts) before making the
		// rootfs readonly (requested by spec.Root.Readonly).
		customopts.WithNewSnapshot(id, containerdImage, snapshotterOpt),
	}
	if len(volumeMounts) > 0 {
		mountMap := make(map[string]string)
		for _, v := range volumeMounts {
			mountMap[filepath.Clean(v.HostPath)] = v.ContainerPath
		}
		opts = append(opts, customopts.WithVolumes(mountMap))
	}
	meta.ImageRef = image.ID
	meta.StopSignal = image.ImageSpec.Config.StopSignal

	// Validate log paths and compose full container log path.
	if sandboxConfig.GetLogDirectory() != "" && config.GetLogPath() != "" {
		meta.LogPath = filepath.Join(sandboxConfig.GetLogDirectory(), config.GetLogPath())
		log.G(ctx).Debugf("Composed container full log path %q using sandbox log dir %q and container log path %q",
			meta.LogPath, sandboxConfig.GetLogDirectory(), config.GetLogPath())
	} else {
		log.G(ctx).Infof("Logging will be disabled due to empty log paths for sandbox (%q) or container (%q)",
			sandboxConfig.GetLogDirectory(), config.GetLogPath())
	}

	containerIO, err := cio.NewContainerIO(id,
		cio.WithNewFIFOs(volatileContainerRootDir, config.GetTty(), config.GetStdin()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create container io")
	}
	defer func() {
		if retErr != nil {
			if err := containerIO.Close(); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to close container io %q", id)
			}
		}
	}()

	specOpts, err := c.containerSpecOpts(config, &image.ImageSpec.Config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get container spec opts")
	}

	containerLabels := buildLabels(config.Labels, containerKindContainer)

	runtimeOptions, err := getRuntimeOptions(sandboxInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get runtime options")
	}
	opts = append(opts,
		containerd.WithSpec(spec, specOpts...),
		containerd.WithRuntime(sandboxInfo.Runtime.Name, runtimeOptions),
		containerd.WithContainerLabels(containerLabels),
		containerd.WithContainerExtension(containerMetadataExtension, &meta))
	var cntr containerd.Container
	if cntr, err = c.client.NewContainer(ctx, id, opts...); err != nil {
		return nil, errors.Wrap(err, "failed to create containerd container")
	}
	defer func() {
		if retErr != nil {
			deferCtx, deferCancel := ctrdutil.DeferContext()
			defer deferCancel()
			if err := cntr.Delete(deferCtx, containerd.WithSnapshotCleanup); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to delete containerd container %q", id)
			}
		}
	}()

	status := containerstore.Status{CreatedAt: time.Now().UnixNano()}
	container, err := containerstore.NewContainer(meta,
		containerstore.WithStatus(status, containerRootDir),
		containerstore.WithContainer(cntr),
		containerstore.WithContainerIO(containerIO),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create internal container object for %q", id)
	}
	defer func() {
		if retErr != nil {
			// Cleanup container checkpoint on error.
			if err := container.Delete(); err != nil {
				log.G(ctx).WithError(err).Errorf("Failed to cleanup container checkpoint for %q", id)
			}
		}
	}()

	// Add container into container store.
	if err := c.containerStore.Add(container); err != nil {
		return nil, errors.Wrapf(err, "failed to add container %q into store", id)
	}

	return &runtime.CreateContainerResponse{ContainerId: id}, nil
}

// volumeMounts sets up image volumes for container. Rely on the removal of container
// root directory to do cleanup. Note that image volume will be skipped, if there is criMounts
// specified with the same destination.
func (c *criService) volumeMounts(containerRootDir string, criMounts []*runtime.Mount, config *imagespec.ImageConfig) []*runtime.Mount {
	if len(config.Volumes) == 0 {
		return nil
	}
	var mounts []*runtime.Mount
	for dst := range config.Volumes {
		if isInCRIMounts(dst, criMounts) {
			// Skip the image volume, if there is CRI defined volume mapping.
			// TODO(random-liu): This should be handled by Kubelet in the future.
			// Kubelet should decide what to use for image volume, and also de-duplicate
			// the image volume and user mounts.
			continue
		}
		volumeID := util.GenerateID()
		src := filepath.Join(containerRootDir, "volumes", volumeID)
		// addOCIBindMounts will create these volumes.
		mounts = append(mounts, &runtime.Mount{
			ContainerPath:  dst,
			HostPath:       src,
			SelinuxRelabel: true,
		})
	}
	return mounts
}

// runtimeSpec returns a default runtime spec used in cri-containerd.
func (c *criService) runtimeSpec(id string, baseSpecFile string, opts ...oci.SpecOpts) (*runtimespec.Spec, error) {
	// GenerateSpec needs namespace.
	ctx := ctrdutil.NamespacedContext()
	container := &containers.Container{ID: id}

	if baseSpecFile != "" {
		baseSpec, ok := c.baseOCISpecs[baseSpecFile]
		if !ok {
			return nil, errors.Errorf("can't find base OCI spec %q", baseSpecFile)
		}

		spec := oci.Spec{}
		if err := util.DeepCopy(&spec, &baseSpec); err != nil {
			return nil, errors.Wrap(err, "failed to clone OCI spec")
		}

		// Fix up cgroups path
		applyOpts := append([]oci.SpecOpts{oci.WithNamespacedCgroup()}, opts...)

		if err := oci.ApplyOpts(ctx, nil, container, &spec, applyOpts...); err != nil {
			return nil, errors.Wrap(err, "failed to apply OCI options")
		}

		return &spec, nil
	}

	spec, err := oci.GenerateSpec(ctx, nil, container, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate spec")
	}

	return spec, nil
}
