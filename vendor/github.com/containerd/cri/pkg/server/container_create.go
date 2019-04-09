/*
Copyright 2017 The Kubernetes Authors.

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
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/typeurl"
	"github.com/davecgh/go-spew/spew"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runc/libcontainer/devices"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/validate"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/gocapability/capability"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/annotations"
	customopts "github.com/containerd/cri/pkg/containerd/opts"
	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	cio "github.com/containerd/cri/pkg/server/io"
	containerstore "github.com/containerd/cri/pkg/store/container"
	"github.com/containerd/cri/pkg/util"
)

const (
	// profileNamePrefix is the prefix for loading profiles on a localhost. Eg. AppArmor localhost/profileName.
	profileNamePrefix = "localhost/" // TODO (mikebrow): get localhost/ & runtime/default from CRI kubernetes/kubernetes#51747
	// runtimeDefault indicates that we should use or create a runtime default profile.
	runtimeDefault = "runtime/default"
	// dockerDefault indicates that we should use or create a docker default profile.
	dockerDefault = "docker/default"
	// appArmorDefaultProfileName is name to use when creating a default apparmor profile.
	appArmorDefaultProfileName = "cri-containerd.apparmor.d"
	// unconfinedProfile is a string indicating one should run a pod/containerd without a security profile
	unconfinedProfile = "unconfined"
	// seccompDefaultProfile is the default seccomp profile.
	seccompDefaultProfile = dockerDefault
)

func init() {
	typeurl.Register(&containerstore.Metadata{},
		"github.com/containerd/cri/pkg/store/container", "Metadata")
}

// CreateContainer creates a new container in the given PodSandbox.
func (c *criService) CreateContainer(ctx context.Context, r *runtime.CreateContainerRequest) (_ *runtime.CreateContainerResponse, retErr error) {
	config := r.GetConfig()
	logrus.Debugf("Container config %+v", config)
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
	name := makeContainerName(config.GetMetadata(), sandboxConfig.GetMetadata())
	logrus.Debugf("Generated id %q for container %q", id, name)
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
				logrus.WithError(err).Errorf("Failed to remove container root directory %q",
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
				logrus.WithError(err).Errorf("Failed to remove volatile container root directory %q",
					volatileContainerRootDir)
			}
		}
	}()

	// Create container volumes mounts.
	volumeMounts := c.generateVolumeMounts(containerRootDir, config.GetMounts(), &image.ImageSpec.Config)

	// Generate container runtime spec.
	mounts := c.generateContainerMounts(sandboxID, config)

	spec, err := c.generateContainerSpec(id, sandboxID, sandboxPid, config, sandboxConfig, &image.ImageSpec.Config, append(mounts, volumeMounts...))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate container %q spec", id)
	}

	logrus.Debugf("Container %q spec: %#+v", id, spew.NewFormatter(spec))

	// Set snapshotter before any other options.
	opts := []containerd.NewContainerOpts{
		containerd.WithSnapshotter(c.config.ContainerdConfig.Snapshotter),
		// Prepare container rootfs. This is always writeable even if
		// the container wants a readonly rootfs since we want to give
		// the runtime (runc) a chance to modify (e.g. to create mount
		// points corresponding to spec.Mounts) before making the
		// rootfs readonly (requested by spec.Root.Readonly).
		customopts.WithNewSnapshot(id, image.Image),
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

	// Get container log path.
	if config.GetLogPath() != "" {
		meta.LogPath = filepath.Join(sandboxConfig.GetLogDirectory(), config.GetLogPath())
	}

	containerIO, err := cio.NewContainerIO(id,
		cio.WithNewFIFOs(volatileContainerRootDir, config.GetTty(), config.GetStdin()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create container io")
	}
	defer func() {
		if retErr != nil {
			if err := containerIO.Close(); err != nil {
				logrus.WithError(err).Errorf("Failed to close container io %q", id)
			}
		}
	}()

	var specOpts []oci.SpecOpts
	securityContext := config.GetLinux().GetSecurityContext()
	// Set container username. This could only be done by containerd, because it needs
	// access to the container rootfs. Pass user name to containerd, and let it overwrite
	// the spec for us.
	userstr, err := generateUserString(
		securityContext.GetRunAsUsername(),
		securityContext.GetRunAsUser(),
		securityContext.GetRunAsGroup(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate user string")
	}
	if userstr != "" {
		specOpts = append(specOpts, oci.WithUser(userstr))
	}

	if securityContext.GetRunAsUsername() != "" {
		userstr = securityContext.GetRunAsUsername()
	} else {
		// Even if RunAsUser is not set, we still call `GetValue` to get uid 0.
		// Because it is still useful to get additional gids for uid 0.
		userstr = strconv.FormatInt(securityContext.GetRunAsUser().GetValue(), 10)
	}
	specOpts = append(specOpts, customopts.WithAdditionalGIDs(userstr))

	apparmorSpecOpts, err := generateApparmorSpecOpts(
		securityContext.GetApparmorProfile(),
		securityContext.GetPrivileged(),
		c.apparmorEnabled)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate apparmor spec opts")
	}
	if apparmorSpecOpts != nil {
		specOpts = append(specOpts, apparmorSpecOpts)
	}

	seccompSpecOpts, err := generateSeccompSpecOpts(
		securityContext.GetSeccompProfilePath(),
		securityContext.GetPrivileged(),
		c.seccompEnabled)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate seccomp spec opts")
	}
	if seccompSpecOpts != nil {
		specOpts = append(specOpts, seccompSpecOpts)
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
				logrus.WithError(err).Errorf("Failed to delete containerd container %q", id)
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
				logrus.WithError(err).Errorf("Failed to cleanup container checkpoint for %q", id)
			}
		}
	}()

	// Add container into container store.
	if err := c.containerStore.Add(container); err != nil {
		return nil, errors.Wrapf(err, "failed to add container %q into store", id)
	}

	return &runtime.CreateContainerResponse{ContainerId: id}, nil
}

func (c *criService) generateContainerSpec(id string, sandboxID string, sandboxPid uint32, config *runtime.ContainerConfig,
	sandboxConfig *runtime.PodSandboxConfig, imageConfig *imagespec.ImageConfig, extraMounts []*runtime.Mount) (*runtimespec.Spec, error) {
	// Creates a spec Generator with the default spec.
	spec, err := defaultRuntimeSpec(id)
	if err != nil {
		return nil, err
	}
	g := newSpecGenerator(spec)

	// Set the relative path to the rootfs of the container from containerd's
	// pre-defined directory.
	g.SetRootPath(relativeRootfsPath)

	if err := setOCIProcessArgs(&g, config, imageConfig); err != nil {
		return nil, err
	}

	if config.GetWorkingDir() != "" {
		g.SetProcessCwd(config.GetWorkingDir())
	} else if imageConfig.WorkingDir != "" {
		g.SetProcessCwd(imageConfig.WorkingDir)
	}

	g.SetProcessTerminal(config.GetTty())
	if config.GetTty() {
		g.AddProcessEnv("TERM", "xterm")
	}

	// Add HOSTNAME env.
	hostname := sandboxConfig.GetHostname()
	if sandboxConfig.GetHostname() == "" {
		hostname, err = c.os.Hostname()
		if err != nil {
			return nil, err
		}
	}
	g.AddProcessEnv(hostnameEnv, hostname)

	// Apply envs from image config first, so that envs from container config
	// can override them.
	if err := addImageEnvs(&g, imageConfig.Env); err != nil {
		return nil, err
	}
	for _, e := range config.GetEnvs() {
		g.AddProcessEnv(e.GetKey(), e.GetValue())
	}

	securityContext := config.GetLinux().GetSecurityContext()
	selinuxOpt := securityContext.GetSelinuxOptions()
	processLabel, mountLabel, err := initSelinuxOpts(selinuxOpt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init selinux options %+v", securityContext.GetSelinuxOptions())
	}

	// Merge extra mounts and CRI mounts.
	mounts := mergeMounts(config.GetMounts(), extraMounts)
	if err := c.addOCIBindMounts(&g, mounts, mountLabel); err != nil {
		return nil, errors.Wrapf(err, "failed to set OCI bind mounts %+v", mounts)
	}

	// Apply masked paths if specified.
	// When `MaskedPaths` is not specified, keep runtime default for backward compatibility;
	// When `MaskedPaths` is specified, but length is zero, clear masked path list.
	if securityContext.GetMaskedPaths() != nil {
		g.Config.Linux.MaskedPaths = nil
		for _, path := range securityContext.GetMaskedPaths() {
			g.AddLinuxMaskedPaths(path)
		}
	}

	// Apply readonly paths if specified.
	if securityContext.GetReadonlyPaths() != nil {
		g.Config.Linux.ReadonlyPaths = nil
		for _, path := range securityContext.GetReadonlyPaths() {
			g.AddLinuxReadonlyPaths(path)
		}
	}

	if securityContext.GetPrivileged() {
		if !sandboxConfig.GetLinux().GetSecurityContext().GetPrivileged() {
			return nil, errors.New("no privileged container allowed in sandbox")
		}
		if err := setOCIPrivileged(&g, config); err != nil {
			return nil, err
		}
	} else { // not privileged
		if err := c.addOCIDevices(&g, config.GetDevices()); err != nil {
			return nil, errors.Wrapf(err, "failed to set devices mapping %+v", config.GetDevices())
		}

		if err := setOCICapabilities(&g, securityContext.GetCapabilities()); err != nil {
			return nil, errors.Wrapf(err, "failed to set capabilities %+v",
				securityContext.GetCapabilities())
		}
	}
	// Clear all ambient capabilities. The implication of non-root + caps
	// is not clearly defined in Kubernetes.
	// See https://github.com/kubernetes/kubernetes/issues/56374
	// Keep docker's behavior for now.
	g.Config.Process.Capabilities.Ambient = []string{}

	g.SetProcessSelinuxLabel(processLabel)
	g.SetLinuxMountLabel(mountLabel)

	// TODO: Figure out whether we should set no new privilege for sandbox container by default
	g.SetProcessNoNewPrivileges(securityContext.GetNoNewPrivs())

	// TODO(random-liu): [P1] Set selinux options (privileged or not).

	g.SetRootReadonly(securityContext.GetReadonlyRootfs())

	setOCILinuxResource(&g, config.GetLinux().GetResources())

	if sandboxConfig.GetLinux().GetCgroupParent() != "" {
		cgroupsPath := getCgroupsPath(sandboxConfig.GetLinux().GetCgroupParent(), id,
			c.config.SystemdCgroup)
		g.SetLinuxCgroupsPath(cgroupsPath)
	}

	// Set namespaces, share namespace with sandbox container.
	setOCINamespaces(&g, securityContext.GetNamespaceOptions(), sandboxPid)

	supplementalGroups := securityContext.GetSupplementalGroups()
	for _, group := range supplementalGroups {
		g.AddProcessAdditionalGid(uint32(group))
	}

	g.AddAnnotation(annotations.ContainerType, annotations.ContainerTypeContainer)
	g.AddAnnotation(annotations.SandboxID, sandboxID)

	return g.Config, nil
}

// generateVolumeMounts sets up image volumes for container. Rely on the removal of container
// root directory to do cleanup. Note that image volume will be skipped, if there is criMounts
// specified with the same destination.
func (c *criService) generateVolumeMounts(containerRootDir string, criMounts []*runtime.Mount, config *imagespec.ImageConfig) []*runtime.Mount {
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
			ContainerPath: dst,
			HostPath:      src,
			// Use default mount propagation.
			// TODO(random-liu): What about selinux relabel?
		})
	}
	return mounts
}

// generateContainerMounts sets up necessary container mounts including /dev/shm, /etc/hosts
// and /etc/resolv.conf.
func (c *criService) generateContainerMounts(sandboxID string, config *runtime.ContainerConfig) []*runtime.Mount {
	var mounts []*runtime.Mount
	securityContext := config.GetLinux().GetSecurityContext()
	if !isInCRIMounts(etcHostname, config.GetMounts()) {
		// /etc/hostname is added since 1.1.6, 1.2.4 and 1.3.
		// For in-place upgrade, the old sandbox doesn't have the hostname file,
		// do not mount this in that case.
		// TODO(random-liu): Remove the check and always mount this when
		// containerd 1.1 and 1.2 are deprecated.
		hostpath := c.getSandboxHostname(sandboxID)
		if _, err := c.os.Stat(hostpath); err == nil {
			mounts = append(mounts, &runtime.Mount{
				ContainerPath: etcHostname,
				HostPath:      hostpath,
				Readonly:      securityContext.GetReadonlyRootfs(),
			})
		}
	}

	if !isInCRIMounts(etcHosts, config.GetMounts()) {
		mounts = append(mounts, &runtime.Mount{
			ContainerPath: etcHosts,
			HostPath:      c.getSandboxHosts(sandboxID),
			Readonly:      securityContext.GetReadonlyRootfs(),
		})
	}

	// Mount sandbox resolv.config.
	// TODO: Need to figure out whether we should always mount it as read-only
	if !isInCRIMounts(resolvConfPath, config.GetMounts()) {
		mounts = append(mounts, &runtime.Mount{
			ContainerPath: resolvConfPath,
			HostPath:      c.getResolvPath(sandboxID),
			Readonly:      securityContext.GetReadonlyRootfs(),
		})
	}

	if !isInCRIMounts(devShm, config.GetMounts()) {
		sandboxDevShm := c.getSandboxDevShm(sandboxID)
		if securityContext.GetNamespaceOptions().GetIpc() == runtime.NamespaceMode_NODE {
			sandboxDevShm = devShm
		}
		mounts = append(mounts, &runtime.Mount{
			ContainerPath: devShm,
			HostPath:      sandboxDevShm,
			Readonly:      false,
		})
	}
	return mounts
}

// setOCIProcessArgs sets process args. It returns error if the final arg list
// is empty.
func setOCIProcessArgs(g *generator, config *runtime.ContainerConfig, imageConfig *imagespec.ImageConfig) error {
	command, args := config.GetCommand(), config.GetArgs()
	// The following logic is migrated from https://github.com/moby/moby/blob/master/daemon/commit.go
	// TODO(random-liu): Clearly define the commands overwrite behavior.
	if len(command) == 0 {
		// Copy array to avoid data race.
		if len(args) == 0 {
			args = append([]string{}, imageConfig.Cmd...)
		}
		if command == nil {
			command = append([]string{}, imageConfig.Entrypoint...)
		}
	}
	if len(command) == 0 && len(args) == 0 {
		return errors.New("no command specified")
	}
	g.SetProcessArgs(append(command, args...))
	return nil
}

// addImageEnvs adds environment variables from image config. It returns error if
// an invalid environment variable is encountered.
func addImageEnvs(g *generator, imageEnvs []string) error {
	for _, e := range imageEnvs {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) != 2 {
			return errors.Errorf("invalid environment variable %q", e)
		}
		g.AddProcessEnv(kv[0], kv[1])
	}
	return nil
}

func setOCIPrivileged(g *generator, config *runtime.ContainerConfig) error {
	// Add all capabilities in privileged mode.
	g.SetupPrivileged(true)
	setOCIBindMountsPrivileged(g)
	if err := setOCIDevicesPrivileged(g); err != nil {
		return errors.Wrapf(err, "failed to set devices mapping %+v", config.GetDevices())
	}
	return nil
}

func clearReadOnly(m *runtimespec.Mount) {
	var opt []string
	for _, o := range m.Options {
		if o != "ro" {
			opt = append(opt, o)
		}
	}
	m.Options = append(opt, "rw")
}

// addDevices set device mapping without privilege.
func (c *criService) addOCIDevices(g *generator, devs []*runtime.Device) error {
	spec := g.Config
	for _, device := range devs {
		path, err := c.os.ResolveSymbolicLink(device.HostPath)
		if err != nil {
			return err
		}
		dev, err := devices.DeviceFromPath(path, device.Permissions)
		if err != nil {
			return err
		}
		rd := runtimespec.LinuxDevice{
			Path:  device.ContainerPath,
			Type:  string(dev.Type),
			Major: dev.Major,
			Minor: dev.Minor,
			UID:   &dev.Uid,
			GID:   &dev.Gid,
		}
		g.AddDevice(rd)
		spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, runtimespec.LinuxDeviceCgroup{
			Allow:  true,
			Type:   string(dev.Type),
			Major:  &dev.Major,
			Minor:  &dev.Minor,
			Access: dev.Permissions,
		})
	}
	return nil
}

// addDevices set device mapping with privilege.
func setOCIDevicesPrivileged(g *generator) error {
	spec := g.Config
	hostDevices, err := devices.HostDevices()
	if err != nil {
		return err
	}
	for _, hostDevice := range hostDevices {
		rd := runtimespec.LinuxDevice{
			Path:  hostDevice.Path,
			Type:  string(hostDevice.Type),
			Major: hostDevice.Major,
			Minor: hostDevice.Minor,
			UID:   &hostDevice.Uid,
			GID:   &hostDevice.Gid,
		}
		if hostDevice.Major == 0 && hostDevice.Minor == 0 {
			// Invalid device, most likely a symbolic link, skip it.
			continue
		}
		g.AddDevice(rd)
	}
	spec.Linux.Resources.Devices = []runtimespec.LinuxDeviceCgroup{
		{
			Allow:  true,
			Access: "rwm",
		},
	}
	return nil
}

// addOCIBindMounts adds bind mounts.
func (c *criService) addOCIBindMounts(g *generator, mounts []*runtime.Mount, mountLabel string) error {
	// Sort mounts in number of parts. This ensures that high level mounts don't
	// shadow other mounts.
	sort.Sort(orderedMounts(mounts))

	// Mount cgroup into the container as readonly, which inherits docker's behavior.
	g.AddMount(runtimespec.Mount{
		Source:      "cgroup",
		Destination: "/sys/fs/cgroup",
		Type:        "cgroup",
		Options:     []string{"nosuid", "noexec", "nodev", "relatime", "ro"},
	})

	// Copy all mounts from default mounts, except for
	// - mounts overriden by supplied mount;
	// - all mounts under /dev if a supplied /dev is present.
	mountSet := make(map[string]struct{})
	for _, m := range mounts {
		mountSet[filepath.Clean(m.ContainerPath)] = struct{}{}
	}
	defaultMounts := g.Mounts()
	g.ClearMounts()
	for _, m := range defaultMounts {
		dst := filepath.Clean(m.Destination)
		if _, ok := mountSet[dst]; ok {
			// filter out mount overridden by a supplied mount
			continue
		}
		if _, mountDev := mountSet["/dev"]; mountDev && strings.HasPrefix(dst, "/dev/") {
			// filter out everything under /dev if /dev is a supplied mount
			continue
		}
		g.AddMount(m)
	}

	for _, mount := range mounts {
		dst := mount.GetContainerPath()
		src := mount.GetHostPath()
		// Create the host path if it doesn't exist.
		// TODO(random-liu): Add CRI validation test for this case.
		if _, err := c.os.Stat(src); err != nil {
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "failed to stat %q", src)
			}
			if err := c.os.MkdirAll(src, 0755); err != nil {
				return errors.Wrapf(err, "failed to mkdir %q", src)
			}
		}
		// TODO(random-liu): Add cri-containerd integration test or cri validation test
		// for this.
		src, err := c.os.ResolveSymbolicLink(src)
		if err != nil {
			return errors.Wrapf(err, "failed to resolve symlink %q", src)
		}

		options := []string{"rbind"}
		switch mount.GetPropagation() {
		case runtime.MountPropagation_PROPAGATION_PRIVATE:
			options = append(options, "rprivate")
			// Since default root propogation in runc is rprivate ignore
			// setting the root propagation
		case runtime.MountPropagation_PROPAGATION_BIDIRECTIONAL:
			if err := ensureShared(src, c.os.LookupMount); err != nil {
				return err
			}
			options = append(options, "rshared")
			g.SetLinuxRootPropagation("rshared") // nolint: errcheck
		case runtime.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
			if err := ensureSharedOrSlave(src, c.os.LookupMount); err != nil {
				return err
			}
			options = append(options, "rslave")
			if g.Config.Linux.RootfsPropagation != "rshared" &&
				g.Config.Linux.RootfsPropagation != "rslave" {
				g.SetLinuxRootPropagation("rslave") // nolint: errcheck
			}
		default:
			logrus.Warnf("Unknown propagation mode for hostPath %q", mount.HostPath)
			options = append(options, "rprivate")
		}

		// NOTE(random-liu): we don't change all mounts to `ro` when root filesystem
		// is readonly. This is different from docker's behavior, but make more sense.
		if mount.GetReadonly() {
			options = append(options, "ro")
		} else {
			options = append(options, "rw")
		}

		if mount.GetSelinuxRelabel() {
			if err := label.Relabel(src, mountLabel, true); err != nil && err != unix.ENOTSUP {
				return errors.Wrapf(err, "relabel %q with %q failed", src, mountLabel)
			}
		}
		g.AddMount(runtimespec.Mount{
			Source:      src,
			Destination: dst,
			Type:        "bind",
			Options:     options,
		})
	}

	return nil
}

func setOCIBindMountsPrivileged(g *generator) {
	spec := g.Config
	// clear readonly for /sys and cgroup
	for i, m := range spec.Mounts {
		if filepath.Clean(spec.Mounts[i].Destination) == "/sys" {
			clearReadOnly(&spec.Mounts[i])
		}
		if m.Type == "cgroup" {
			clearReadOnly(&spec.Mounts[i])
		}
	}
	spec.Linux.ReadonlyPaths = nil
	spec.Linux.MaskedPaths = nil
}

// setOCILinuxResource set container resource limit.
func setOCILinuxResource(g *generator, resources *runtime.LinuxContainerResources) {
	if resources == nil {
		return
	}
	g.SetLinuxResourcesCPUPeriod(uint64(resources.GetCpuPeriod()))
	g.SetLinuxResourcesCPUQuota(resources.GetCpuQuota())
	g.SetLinuxResourcesCPUShares(uint64(resources.GetCpuShares()))
	g.SetLinuxResourcesMemoryLimit(resources.GetMemoryLimitInBytes())
	g.SetProcessOOMScoreAdj(int(resources.GetOomScoreAdj()))
	g.SetLinuxResourcesCPUCpus(resources.GetCpusetCpus())
	g.SetLinuxResourcesCPUMems(resources.GetCpusetMems())
}

// getOCICapabilitiesList returns a list of all available capabilities.
func getOCICapabilitiesList() []string {
	var caps []string
	for _, cap := range capability.List() {
		if cap > validate.LastCap() {
			continue
		}
		caps = append(caps, "CAP_"+strings.ToUpper(cap.String()))
	}
	return caps
}

// Adds capabilities to all sets relevant to root (bounding, permitted, effective, inheritable)
func addProcessRootCapability(g *generator, c string) error {
	if err := g.AddProcessCapabilityBounding(c); err != nil {
		return err
	}
	if err := g.AddProcessCapabilityPermitted(c); err != nil {
		return err
	}
	if err := g.AddProcessCapabilityEffective(c); err != nil {
		return err
	}
	if err := g.AddProcessCapabilityInheritable(c); err != nil {
		return err
	}
	return nil
}

// Drops capabilities to all sets relevant to root (bounding, permitted, effective, inheritable)
func dropProcessRootCapability(g *generator, c string) error {
	if err := g.DropProcessCapabilityBounding(c); err != nil {
		return err
	}
	if err := g.DropProcessCapabilityPermitted(c); err != nil {
		return err
	}
	if err := g.DropProcessCapabilityEffective(c); err != nil {
		return err
	}
	if err := g.DropProcessCapabilityInheritable(c); err != nil {
		return err
	}
	return nil
}

// setOCICapabilities adds/drops process capabilities.
func setOCICapabilities(g *generator, capabilities *runtime.Capability) error {
	if capabilities == nil {
		return nil
	}

	// Add/drop all capabilities if "all" is specified, so that
	// following individual add/drop could still work. E.g.
	// AddCapabilities: []string{"ALL"}, DropCapabilities: []string{"CHOWN"}
	// will be all capabilities without `CAP_CHOWN`.
	if util.InStringSlice(capabilities.GetAddCapabilities(), "ALL") {
		for _, c := range getOCICapabilitiesList() {
			if err := addProcessRootCapability(g, c); err != nil {
				return err
			}
		}
	}
	if util.InStringSlice(capabilities.GetDropCapabilities(), "ALL") {
		for _, c := range getOCICapabilitiesList() {
			if err := dropProcessRootCapability(g, c); err != nil {
				return err
			}
		}
	}

	for _, c := range capabilities.GetAddCapabilities() {
		if strings.ToUpper(c) == "ALL" {
			continue
		}
		// Capabilities in CRI doesn't have `CAP_` prefix, so add it.
		if err := addProcessRootCapability(g, "CAP_"+strings.ToUpper(c)); err != nil {
			return err
		}
	}

	for _, c := range capabilities.GetDropCapabilities() {
		if strings.ToUpper(c) == "ALL" {
			continue
		}
		if err := dropProcessRootCapability(g, "CAP_"+strings.ToUpper(c)); err != nil {
			return err
		}
	}
	return nil
}

// setOCINamespaces sets namespaces.
func setOCINamespaces(g *generator, namespaces *runtime.NamespaceOption, sandboxPid uint32) {
	g.AddOrReplaceLinuxNamespace(string(runtimespec.NetworkNamespace), getNetworkNamespace(sandboxPid)) // nolint: errcheck
	g.AddOrReplaceLinuxNamespace(string(runtimespec.IPCNamespace), getIPCNamespace(sandboxPid))         // nolint: errcheck
	g.AddOrReplaceLinuxNamespace(string(runtimespec.UTSNamespace), getUTSNamespace(sandboxPid))         // nolint: errcheck
	// Do not share pid namespace if namespace mode is CONTAINER.
	if namespaces.GetPid() != runtime.NamespaceMode_CONTAINER {
		g.AddOrReplaceLinuxNamespace(string(runtimespec.PIDNamespace), getPIDNamespace(sandboxPid)) // nolint: errcheck
	}
}

// defaultRuntimeSpec returns a default runtime spec used in cri-containerd.
func defaultRuntimeSpec(id string) (*runtimespec.Spec, error) {
	// GenerateSpec needs namespace.
	ctx := ctrdutil.NamespacedContext()
	spec, err := oci.GenerateSpec(ctx, nil, &containers.Container{ID: id})
	if err != nil {
		return nil, err
	}

	// Remove `/run` mount
	// TODO(random-liu): Mount tmpfs for /run and handle copy-up.
	var mounts []runtimespec.Mount
	for _, mount := range spec.Mounts {
		if filepath.Clean(mount.Destination) == "/run" {
			continue
		}
		mounts = append(mounts, mount)
	}
	spec.Mounts = mounts

	// Make sure no default seccomp/apparmor is specified
	if spec.Process != nil {
		spec.Process.ApparmorProfile = ""
	}
	if spec.Linux != nil {
		spec.Linux.Seccomp = nil
	}

	// Remove default rlimits (See issue #515)
	spec.Process.Rlimits = nil

	return spec, nil
}

// generateSeccompSpecOpts generates containerd SpecOpts for seccomp.
func generateSeccompSpecOpts(seccompProf string, privileged, seccompEnabled bool) (oci.SpecOpts, error) {
	if privileged {
		// Do not set seccomp profile when container is privileged
		return nil, nil
	}
	// Set seccomp profile
	if seccompProf == runtimeDefault || seccompProf == dockerDefault {
		// use correct default profile (Eg. if not configured otherwise, the default is docker/default)
		seccompProf = seccompDefaultProfile
	}
	if !seccompEnabled {
		if seccompProf != "" && seccompProf != unconfinedProfile {
			return nil, errors.New("seccomp is not supported")
		}
		return nil, nil
	}
	switch seccompProf {
	case "", unconfinedProfile:
		// Do not set seccomp profile.
		return nil, nil
	case dockerDefault:
		// Note: WithDefaultProfile specOpts must be added after capabilities
		return seccomp.WithDefaultProfile(), nil
	default:
		// Require and Trim default profile name prefix
		if !strings.HasPrefix(seccompProf, profileNamePrefix) {
			return nil, errors.Errorf("invalid seccomp profile %q", seccompProf)
		}
		return seccomp.WithProfile(strings.TrimPrefix(seccompProf, profileNamePrefix)), nil
	}
}

// generateApparmorSpecOpts generates containerd SpecOpts for apparmor.
func generateApparmorSpecOpts(apparmorProf string, privileged, apparmorEnabled bool) (oci.SpecOpts, error) {
	if !apparmorEnabled {
		// Should fail loudly if user try to specify apparmor profile
		// but we don't support it.
		if apparmorProf != "" && apparmorProf != unconfinedProfile {
			return nil, errors.New("apparmor is not supported")
		}
		return nil, nil
	}
	switch apparmorProf {
	case runtimeDefault:
		// TODO (mikebrow): delete created apparmor default profile
		return apparmor.WithDefaultProfile(appArmorDefaultProfileName), nil
	case unconfinedProfile:
		return nil, nil
	case "":
		// Based on kubernetes#51746, default apparmor profile should be applied
		// for non-privileged container when apparmor is not specified.
		if privileged {
			return nil, nil
		}
		return apparmor.WithDefaultProfile(appArmorDefaultProfileName), nil
	default:
		// Require and Trim default profile name prefix
		if !strings.HasPrefix(apparmorProf, profileNamePrefix) {
			return nil, errors.Errorf("invalid apparmor profile %q", apparmorProf)
		}
		return apparmor.WithProfile(strings.TrimPrefix(apparmorProf, profileNamePrefix)), nil
	}
}

// Ensure mount point on which path is mounted, is shared.
func ensureShared(path string, lookupMount func(string) (mount.Info, error)) error {
	mountInfo, err := lookupMount(path)
	if err != nil {
		return err
	}

	// Make sure source mount point is shared.
	optsSplit := strings.Split(mountInfo.Optional, " ")
	for _, opt := range optsSplit {
		if strings.HasPrefix(opt, "shared:") {
			return nil
		}
	}

	return errors.Errorf("path %q is mounted on %q but it is not a shared mount", path, mountInfo.Mountpoint)
}

// Ensure mount point on which path is mounted, is either shared or slave.
func ensureSharedOrSlave(path string, lookupMount func(string) (mount.Info, error)) error {
	mountInfo, err := lookupMount(path)
	if err != nil {
		return err
	}
	// Make sure source mount point is shared.
	optsSplit := strings.Split(mountInfo.Optional, " ")
	for _, opt := range optsSplit {
		if strings.HasPrefix(opt, "shared:") {
			return nil
		} else if strings.HasPrefix(opt, "master:") {
			return nil
		}
	}
	return errors.Errorf("path %q is mounted on %q but it is not a shared or slave mount", path, mountInfo.Mountpoint)
}

// generateUserString generates valid user string based on OCI Image Spec v1.0.0.
// TODO(random-liu): Add group name support in CRI.
func generateUserString(username string, uid, gid *runtime.Int64Value) (string, error) {
	var userstr, groupstr string
	if uid != nil {
		userstr = strconv.FormatInt(uid.GetValue(), 10)
	}
	if username != "" {
		userstr = username
	}
	if gid != nil {
		groupstr = strconv.FormatInt(gid.GetValue(), 10)
	}
	if userstr == "" {
		if groupstr != "" {
			return "", errors.Errorf("user group %q is specified without user", groupstr)
		}
		return "", nil
	}
	if groupstr != "" {
		userstr = userstr + ":" + groupstr
	}
	return userstr, nil
}

// mergeMounts merge CRI mounts with extra mounts. If a mount destination
// is mounted by both a CRI mount and an extra mount, the CRI mount will
// be kept.
func mergeMounts(criMounts, extraMounts []*runtime.Mount) []*runtime.Mount {
	var mounts []*runtime.Mount
	mounts = append(mounts, criMounts...)
	// Copy all mounts from extra mounts, except for mounts overriden by CRI.
	for _, e := range extraMounts {
		found := false
		for _, c := range criMounts {
			if filepath.Clean(e.ContainerPath) == filepath.Clean(c.ContainerPath) {
				found = true
				break
			}
		}
		if !found {
			mounts = append(mounts, e)
		}
	}
	return mounts
}
