/*
Copyright 2018 The containerd Authors.

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

package opts

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/oci"

	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/runc/libcontainer/devices"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	osinterface "github.com/containerd/cri/pkg/os"
	"github.com/containerd/cri/pkg/util"
)

const (
	// DefaultSandboxCPUshares is default cpu shares for sandbox container.
	DefaultSandboxCPUshares = 2
)

// WithAdditionalGIDs adds any additional groups listed for a particular user in the
// /etc/groups file of the image's root filesystem to the OCI spec's additionalGids array.
func WithAdditionalGIDs(userstr string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		if s.Process == nil {
			s.Process = &runtimespec.Process{}
		}
		gids := s.Process.User.AdditionalGids
		if err := oci.WithAdditionalGIDs(userstr)(ctx, client, c, s); err != nil {
			return err
		}
		// Merge existing gids and new gids.
		s.Process.User.AdditionalGids = mergeGids(s.Process.User.AdditionalGids, gids)
		return nil
	}
}

func mergeGids(gids1, gids2 []uint32) []uint32 {
	gidsMap := make(map[uint32]struct{})
	for _, gid1 := range gids1 {
		gidsMap[gid1] = struct{}{}
	}
	for _, gid2 := range gids2 {
		gidsMap[gid2] = struct{}{}
	}
	var gids []uint32
	for gid := range gidsMap {
		gids = append(gids, gid)
	}
	sort.Slice(gids, func(i, j int) bool { return gids[i] < gids[j] })
	return gids
}

// WithoutRunMount removes the `/run` inside the spec
func WithoutRunMount(_ context.Context, _ oci.Client, c *containers.Container, s *runtimespec.Spec) error {
	var (
		mounts  []runtimespec.Mount
		current = s.Mounts
	)
	for _, m := range current {
		if filepath.Clean(m.Destination) == "/run" {
			continue
		}
		mounts = append(mounts, m)
	}
	s.Mounts = mounts
	return nil
}

// WithoutDefaultSecuritySettings removes the default security settings generated on a spec
func WithoutDefaultSecuritySettings(_ context.Context, _ oci.Client, c *containers.Container, s *runtimespec.Spec) error {
	if s.Process == nil {
		s.Process = &runtimespec.Process{}
	}
	// Make sure no default seccomp/apparmor is specified
	s.Process.ApparmorProfile = ""
	if s.Linux != nil {
		s.Linux.Seccomp = nil
	}
	// Remove default rlimits (See issue #515)
	s.Process.Rlimits = nil
	return nil
}

// WithRelativeRoot sets the root for the container
func WithRelativeRoot(root string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		if s.Root == nil {
			s.Root = &runtimespec.Root{}
		}
		s.Root.Path = root
		return nil
	}
}

// WithProcessArgs sets the process args on the spec based on the image and runtime config
func WithProcessArgs(config *runtime.ContainerConfig, image *imagespec.ImageConfig) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		command, args := config.GetCommand(), config.GetArgs()
		// The following logic is migrated from https://github.com/moby/moby/blob/master/daemon/commit.go
		// TODO(random-liu): Clearly define the commands overwrite behavior.
		if len(command) == 0 {
			// Copy array to avoid data race.
			if len(args) == 0 {
				args = append([]string{}, image.Cmd...)
			}
			if command == nil {
				command = append([]string{}, image.Entrypoint...)
			}
		}
		if len(command) == 0 && len(args) == 0 {
			return errors.New("no command specified")
		}
		return oci.WithProcessArgs(append(command, args...)...)(ctx, client, c, s)
	}
}

// WithMounts sorts and adds runtime and CRI mounts to the spec
func WithMounts(osi osinterface.OS, config *runtime.ContainerConfig, extra []*runtime.Mount, mountLabel string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, _ *containers.Container, s *runtimespec.Spec) (err error) {
		// mergeMounts merge CRI mounts with extra mounts. If a mount destination
		// is mounted by both a CRI mount and an extra mount, the CRI mount will
		// be kept.
		var (
			criMounts = config.GetMounts()
			mounts    = append([]*runtime.Mount{}, criMounts...)
		)
		// Copy all mounts from extra mounts, except for mounts overriden by CRI.
		for _, e := range extra {
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
		// ---

		// Sort mounts in number of parts. This ensures that high level mounts don't
		// shadow other mounts.
		sort.Sort(orderedMounts(mounts))

		// Mount cgroup into the container as readonly, which inherits docker's behavior.
		s.Mounts = append(s.Mounts, runtimespec.Mount{
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

		defaultMounts := s.Mounts
		s.Mounts = nil

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
			s.Mounts = append(s.Mounts, m)
		}

		for _, mount := range mounts {
			var (
				dst = mount.GetContainerPath()
				src = mount.GetHostPath()
			)
			// Create the host path if it doesn't exist.
			// TODO(random-liu): Add CRI validation test for this case.
			if _, err := osi.Stat(src); err != nil {
				if !os.IsNotExist(err) {
					return errors.Wrapf(err, "failed to stat %q", src)
				}
				if err := osi.MkdirAll(src, 0755); err != nil {
					return errors.Wrapf(err, "failed to mkdir %q", src)
				}
			}
			// TODO(random-liu): Add cri-containerd integration test or cri validation test
			// for this.
			src, err := osi.ResolveSymbolicLink(src)
			if err != nil {
				return errors.Wrapf(err, "failed to resolve symlink %q", src)
			}
			if s.Linux == nil {
				s.Linux = &runtimespec.Linux{}
			}
			options := []string{"rbind"}
			switch mount.GetPropagation() {
			case runtime.MountPropagation_PROPAGATION_PRIVATE:
				options = append(options, "rprivate")
				// Since default root propogation in runc is rprivate ignore
				// setting the root propagation
			case runtime.MountPropagation_PROPAGATION_BIDIRECTIONAL:
				if err := ensureShared(src, osi.LookupMount); err != nil {
					return err
				}
				options = append(options, "rshared")
				s.Linux.RootfsPropagation = "rshared"
			case runtime.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
				if err := ensureSharedOrSlave(src, osi.LookupMount); err != nil {
					return err
				}
				options = append(options, "rslave")
				if s.Linux.RootfsPropagation != "rshared" &&
					s.Linux.RootfsPropagation != "rslave" {
					s.Linux.RootfsPropagation = "rslave"
				}
			default:
				log.G(ctx).Warnf("Unknown propagation mode for hostPath %q", mount.HostPath)
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
			s.Mounts = append(s.Mounts, runtimespec.Mount{
				Source:      src,
				Destination: dst,
				Type:        "bind",
				Options:     options,
			})
		}
		return nil
	}
}

// mounts defines how to sort runtime.Mount.
// This is the same with the Docker implementation:
//   https://github.com/moby/moby/blob/17.05.x/daemon/volumes.go#L26
type orderedMounts []*runtime.Mount

// Len returns the number of mounts. Used in sorting.
func (m orderedMounts) Len() int {
	return len(m)
}

// Less returns true if the number of parts (a/b/c would be 3 parts) in the
// mount indexed by parameter 1 is less than that of the mount indexed by
// parameter 2. Used in sorting.
func (m orderedMounts) Less(i, j int) bool {
	return m.parts(i) < m.parts(j)
}

// Swap swaps two items in an array of mounts. Used in sorting
func (m orderedMounts) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

// parts returns the number of parts in the destination of a mount. Used in sorting.
func (m orderedMounts) parts(i int) int {
	return strings.Count(filepath.Clean(m[i].ContainerPath), string(os.PathSeparator))
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

// ensure mount point on which path is mounted, is either shared or slave.
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

// WithPrivilegedDevices allows all host devices inside the container
func WithPrivilegedDevices(_ context.Context, _ oci.Client, _ *containers.Container, s *runtimespec.Spec) error {
	if s.Linux == nil {
		s.Linux = &runtimespec.Linux{}
	}
	if s.Linux.Resources == nil {
		s.Linux.Resources = &runtimespec.LinuxResources{}
	}
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
		addDevice(s, rd)
	}
	s.Linux.Resources.Devices = []runtimespec.LinuxDeviceCgroup{
		{
			Allow:  true,
			Access: "rwm",
		},
	}
	return nil
}

func addDevice(s *runtimespec.Spec, rd runtimespec.LinuxDevice) {
	for i, dev := range s.Linux.Devices {
		if dev.Path == rd.Path {
			s.Linux.Devices[i] = rd
			return
		}
	}
	s.Linux.Devices = append(s.Linux.Devices, rd)
}

// WithDevices sets the provided devices onto the container spec
func WithDevices(osi osinterface.OS, config *runtime.ContainerConfig) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		if s.Linux == nil {
			s.Linux = &runtimespec.Linux{}
		}
		if s.Linux.Resources == nil {
			s.Linux.Resources = &runtimespec.LinuxResources{}
		}
		for _, device := range config.GetDevices() {
			path, err := osi.ResolveSymbolicLink(device.HostPath)
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

			addDevice(s, rd)

			s.Linux.Resources.Devices = append(s.Linux.Resources.Devices, runtimespec.LinuxDeviceCgroup{
				Allow:  true,
				Type:   string(dev.Type),
				Major:  &dev.Major,
				Minor:  &dev.Minor,
				Access: dev.Permissions,
			})
		}
		return nil
	}
}

// WithCapabilities sets the provided capabilties from the security context
func WithCapabilities(sc *runtime.LinuxContainerSecurityContext) oci.SpecOpts {
	capabilities := sc.GetCapabilities()
	if capabilities == nil {
		return nullOpt
	}

	var opts []oci.SpecOpts
	// Add/drop all capabilities if "all" is specified, so that
	// following individual add/drop could still work. E.g.
	// AddCapabilities: []string{"ALL"}, DropCapabilities: []string{"CHOWN"}
	// will be all capabilities without `CAP_CHOWN`.
	if util.InStringSlice(capabilities.GetAddCapabilities(), "ALL") {
		opts = append(opts, oci.WithAllCapabilities)
	}
	if util.InStringSlice(capabilities.GetDropCapabilities(), "ALL") {
		opts = append(opts, oci.WithCapabilities(nil))
	}

	var caps []string
	for _, c := range capabilities.GetAddCapabilities() {
		if strings.ToUpper(c) == "ALL" {
			continue
		}
		// Capabilities in CRI doesn't have `CAP_` prefix, so add it.
		caps = append(caps, "CAP_"+strings.ToUpper(c))
	}
	opts = append(opts, oci.WithAddedCapabilities(caps))

	caps = []string{}
	for _, c := range capabilities.GetDropCapabilities() {
		if strings.ToUpper(c) == "ALL" {
			continue
		}
		caps = append(caps, "CAP_"+strings.ToUpper(c))
	}
	opts = append(opts, oci.WithDroppedCapabilities(caps))
	return oci.Compose(opts...)
}

// WithoutAmbientCaps removes the ambient caps from the spec
func WithoutAmbientCaps(_ context.Context, _ oci.Client, c *containers.Container, s *runtimespec.Spec) error {
	if s.Process == nil {
		s.Process = &runtimespec.Process{}
	}
	if s.Process.Capabilities == nil {
		s.Process.Capabilities = &runtimespec.LinuxCapabilities{}
	}
	s.Process.Capabilities.Ambient = nil
	return nil
}

// WithDisabledCgroups clears the Cgroups Path from the spec
func WithDisabledCgroups(_ context.Context, _ oci.Client, c *containers.Container, s *runtimespec.Spec) error {
	if s.Linux == nil {
		s.Linux = &runtimespec.Linux{}
	}
	s.Linux.CgroupsPath = ""
	return nil
}

// WithSelinuxLabels sets the mount and process labels
func WithSelinuxLabels(process, mount string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		if s.Linux == nil {
			s.Linux = &runtimespec.Linux{}
		}
		if s.Process == nil {
			s.Process = &runtimespec.Process{}
		}
		s.Linux.MountLabel = mount
		s.Process.SelinuxLabel = process
		return nil
	}
}

// WithResources sets the provided resource restrictions
func WithResources(resources *runtime.LinuxContainerResources) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		if resources == nil {
			return nil
		}
		if s.Linux == nil {
			s.Linux = &runtimespec.Linux{}
		}
		if s.Linux.Resources == nil {
			s.Linux.Resources = &runtimespec.LinuxResources{}
		}
		if s.Linux.Resources.CPU == nil {
			s.Linux.Resources.CPU = &runtimespec.LinuxCPU{}
		}
		if s.Linux.Resources.Memory == nil {
			s.Linux.Resources.Memory = &runtimespec.LinuxMemory{}
		}
		var (
			p      = uint64(resources.GetCpuPeriod())
			q      = resources.GetCpuQuota()
			shares = uint64(resources.GetCpuShares())
			limit  = resources.GetMemoryLimitInBytes()
		)

		if p != 0 {
			s.Linux.Resources.CPU.Period = &p
		}
		if q != 0 {
			s.Linux.Resources.CPU.Quota = &q
		}
		if shares != 0 {
			s.Linux.Resources.CPU.Shares = &shares
		}
		if cpus := resources.GetCpusetCpus(); cpus != "" {
			s.Linux.Resources.CPU.Cpus = cpus
		}
		if mems := resources.GetCpusetMems(); mems != "" {
			s.Linux.Resources.CPU.Mems = resources.GetCpusetMems()
		}
		if limit != 0 {
			s.Linux.Resources.Memory.Limit = &limit
		}
		return nil
	}
}

// WithOOMScoreAdj sets the oom score
func WithOOMScoreAdj(config *runtime.ContainerConfig, restrict bool) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
		if s.Process == nil {
			s.Process = &runtimespec.Process{}
		}

		resources := config.GetLinux().GetResources()
		if resources == nil {
			return nil
		}
		adj := int(resources.GetOomScoreAdj())
		if restrict {
			var err error
			adj, err = restrictOOMScoreAdj(adj)
			if err != nil {
				return err
			}
		}
		s.Process.OOMScoreAdj = &adj
		return nil
	}
}

// WithSysctls sets the provided sysctls onto the spec
func WithSysctls(sysctls map[string]string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
		if s.Linux == nil {
			s.Linux = &runtimespec.Linux{}
		}
		if s.Linux.Sysctl == nil {
			s.Linux.Sysctl = make(map[string]string)
		}
		for k, v := range sysctls {
			s.Linux.Sysctl[k] = v
		}
		return nil
	}
}

// WithPodOOMScoreAdj sets the oom score for the pod sandbox
func WithPodOOMScoreAdj(adj int, restrict bool) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
		if s.Process == nil {
			s.Process = &runtimespec.Process{}
		}
		if restrict {
			var err error
			adj, err = restrictOOMScoreAdj(adj)
			if err != nil {
				return err
			}
		}
		s.Process.OOMScoreAdj = &adj
		return nil
	}
}

// WithSupplementalGroups sets the supplemental groups for the process
func WithSupplementalGroups(groups []int64) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
		if s.Process == nil {
			s.Process = &runtimespec.Process{}
		}
		var guids []uint32
		for _, g := range groups {
			guids = append(guids, uint32(g))
		}
		s.Process.User.AdditionalGids = mergeGids(s.Process.User.AdditionalGids, guids)
		return nil
	}
}

// WithAnnotation sets the provided annotation
func WithAnnotation(k, v string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		s.Annotations[k] = v
		return nil
	}
}

// WithPodNamespaces sets the pod namespaces for the container
func WithPodNamespaces(config *runtime.LinuxContainerSecurityContext, pid uint32) oci.SpecOpts {
	namespaces := config.GetNamespaceOptions()

	opts := []oci.SpecOpts{
		oci.WithLinuxNamespace(runtimespec.LinuxNamespace{Type: runtimespec.NetworkNamespace, Path: GetNetworkNamespace(pid)}),
		oci.WithLinuxNamespace(runtimespec.LinuxNamespace{Type: runtimespec.IPCNamespace, Path: GetIPCNamespace(pid)}),
		oci.WithLinuxNamespace(runtimespec.LinuxNamespace{Type: runtimespec.UTSNamespace, Path: GetUTSNamespace(pid)}),
	}
	if namespaces.GetPid() != runtime.NamespaceMode_CONTAINER {
		opts = append(opts, oci.WithLinuxNamespace(runtimespec.LinuxNamespace{Type: runtimespec.PIDNamespace, Path: GetPIDNamespace(pid)}))
	}
	return oci.Compose(opts...)
}

// WithDefaultSandboxShares sets the default sandbox CPU shares
func WithDefaultSandboxShares(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
	if s.Linux == nil {
		s.Linux = &runtimespec.Linux{}
	}
	if s.Linux.Resources == nil {
		s.Linux.Resources = &runtimespec.LinuxResources{}
	}
	if s.Linux.Resources.CPU == nil {
		s.Linux.Resources.CPU = &runtimespec.LinuxCPU{}
	}
	i := uint64(DefaultSandboxCPUshares)
	s.Linux.Resources.CPU.Shares = &i
	return nil
}

// WithoutNamespace removes the provided namespace
func WithoutNamespace(t runtimespec.LinuxNamespaceType) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) error {
		if s.Linux == nil {
			return nil
		}
		var namespaces []runtimespec.LinuxNamespace
		for i, ns := range s.Linux.Namespaces {
			if ns.Type != t {
				namespaces = append(namespaces, s.Linux.Namespaces[i])
			}
		}
		s.Linux.Namespaces = namespaces
		return nil
	}
}

func nullOpt(_ context.Context, _ oci.Client, _ *containers.Container, _ *runtimespec.Spec) error {
	return nil
}

func getCurrentOOMScoreAdj() (int, error) {
	b, err := ioutil.ReadFile("/proc/self/oom_score_adj")
	if err != nil {
		return 0, errors.Wrap(err, "could not get the daemon oom_score_adj")
	}
	s := strings.TrimSpace(string(b))
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.Wrap(err, "could not get the daemon oom_score_adj")
	}
	return i, nil
}

func restrictOOMScoreAdj(preferredOOMScoreAdj int) (int, error) {
	currentOOMScoreAdj, err := getCurrentOOMScoreAdj()
	if err != nil {
		return preferredOOMScoreAdj, err
	}
	if preferredOOMScoreAdj < currentOOMScoreAdj {
		return currentOOMScoreAdj, nil
	}
	return preferredOOMScoreAdj, nil
}

const (
	// netNSFormat is the format of network namespace of a process.
	netNSFormat = "/proc/%v/ns/net"
	// ipcNSFormat is the format of ipc namespace of a process.
	ipcNSFormat = "/proc/%v/ns/ipc"
	// utsNSFormat is the format of uts namespace of a process.
	utsNSFormat = "/proc/%v/ns/uts"
	// pidNSFormat is the format of pid namespace of a process.
	pidNSFormat = "/proc/%v/ns/pid"
)

// GetNetworkNamespace returns the network namespace of a process.
func GetNetworkNamespace(pid uint32) string {
	return fmt.Sprintf(netNSFormat, pid)
}

// GetIPCNamespace returns the ipc namespace of a process.
func GetIPCNamespace(pid uint32) string {
	return fmt.Sprintf(ipcNSFormat, pid)
}

// GetUTSNamespace returns the uts namespace of a process.
func GetUTSNamespace(pid uint32) string {
	return fmt.Sprintf(utsNSFormat, pid)
}

// GetPIDNamespace returns the pid namespace of a process.
func GetPIDNamespace(pid uint32) string {
	return fmt.Sprintf(pidNSFormat, pid)
}
