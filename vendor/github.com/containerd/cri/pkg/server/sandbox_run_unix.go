// +build !windows

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
	"fmt"
	"os"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/plugin"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/annotations"
	customopts "github.com/containerd/cri/pkg/containerd/opts"
	osinterface "github.com/containerd/cri/pkg/os"
)

func (c *criService) sandboxContainerSpec(id string, config *runtime.PodSandboxConfig,
	imageConfig *imagespec.ImageConfig, nsPath string, runtimePodAnnotations []string) (_ *runtimespec.Spec, retErr error) {
	// Creates a spec Generator with the default spec.
	// TODO(random-liu): [P1] Compare the default settings with docker and containerd default.
	specOpts := []oci.SpecOpts{
		customopts.WithoutRunMount,
		customopts.WithoutDefaultSecuritySettings,
		customopts.WithRelativeRoot(relativeRootfsPath),
		oci.WithEnv(imageConfig.Env),
		oci.WithRootFSReadonly(),
		oci.WithHostname(config.GetHostname()),
	}
	if imageConfig.WorkingDir != "" {
		specOpts = append(specOpts, oci.WithProcessCwd(imageConfig.WorkingDir))
	}

	if len(imageConfig.Entrypoint) == 0 && len(imageConfig.Cmd) == 0 {
		// Pause image must have entrypoint or cmd.
		return nil, errors.Errorf("invalid empty entrypoint and cmd in image config %+v", imageConfig)
	}
	specOpts = append(specOpts, oci.WithProcessArgs(append(imageConfig.Entrypoint, imageConfig.Cmd...)...))

	// Set cgroups parent.
	if c.config.DisableCgroup {
		specOpts = append(specOpts, customopts.WithDisabledCgroups)
	} else {
		if config.GetLinux().GetCgroupParent() != "" {
			cgroupsPath := getCgroupsPath(config.GetLinux().GetCgroupParent(), id)
			specOpts = append(specOpts, oci.WithCgroup(cgroupsPath))
		}
	}

	// When cgroup parent is not set, containerd-shim will create container in a child cgroup
	// of the cgroup itself is in.
	// TODO(random-liu): [P2] Set default cgroup path if cgroup parent is not specified.

	// Set namespace options.
	var (
		securityContext = config.GetLinux().GetSecurityContext()
		nsOptions       = securityContext.GetNamespaceOptions()
	)
	if nsOptions.GetNetwork() == runtime.NamespaceMode_NODE {
		specOpts = append(specOpts, customopts.WithoutNamespace(runtimespec.NetworkNamespace))
		specOpts = append(specOpts, customopts.WithoutNamespace(runtimespec.UTSNamespace))
	} else {
		specOpts = append(specOpts, oci.WithLinuxNamespace(
			runtimespec.LinuxNamespace{
				Type: runtimespec.NetworkNamespace,
				Path: nsPath,
			}))
	}
	if nsOptions.GetPid() == runtime.NamespaceMode_NODE {
		specOpts = append(specOpts, customopts.WithoutNamespace(runtimespec.PIDNamespace))
	}
	if nsOptions.GetIpc() == runtime.NamespaceMode_NODE {
		specOpts = append(specOpts, customopts.WithoutNamespace(runtimespec.IPCNamespace))
	}

	// It's fine to generate the spec before the sandbox /dev/shm
	// is actually created.
	sandboxDevShm := c.getSandboxDevShm(id)
	if nsOptions.GetIpc() == runtime.NamespaceMode_NODE {
		sandboxDevShm = devShm
	}
	specOpts = append(specOpts, oci.WithMounts([]runtimespec.Mount{
		{
			Source:      sandboxDevShm,
			Destination: devShm,
			Type:        "bind",
			Options:     []string{"rbind", "ro"},
		},
		// Add resolv.conf for katacontainers to setup the DNS of pod VM properly.
		{
			Source:      c.getResolvPath(id),
			Destination: resolvConfPath,
			Type:        "bind",
			Options:     []string{"rbind", "ro"},
		},
	}))

	processLabel, mountLabel, err := initLabelsFromOpt(securityContext.GetSelinuxOptions())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init selinux options %+v", securityContext.GetSelinuxOptions())
	}
	defer func() {
		if retErr != nil {
			selinux.ReleaseLabel(processLabel)
		}
	}()

	supplementalGroups := securityContext.GetSupplementalGroups()
	specOpts = append(specOpts,
		customopts.WithSelinuxLabels(processLabel, mountLabel),
		customopts.WithSupplementalGroups(supplementalGroups),
	)

	// Add sysctls
	sysctls := config.GetLinux().GetSysctls()
	specOpts = append(specOpts, customopts.WithSysctls(sysctls))

	// Note: LinuxSandboxSecurityContext does not currently provide an apparmor profile

	if !c.config.DisableCgroup {
		specOpts = append(specOpts, customopts.WithDefaultSandboxShares)
	}
	specOpts = append(specOpts, customopts.WithPodOOMScoreAdj(int(defaultSandboxOOMAdj), c.config.RestrictOOMScoreAdj))

	for pKey, pValue := range getPassthroughAnnotations(config.Annotations,
		runtimePodAnnotations) {
		specOpts = append(specOpts, customopts.WithAnnotation(pKey, pValue))
	}

	specOpts = append(specOpts,
		customopts.WithAnnotation(annotations.ContainerType, annotations.ContainerTypeSandbox),
		customopts.WithAnnotation(annotations.SandboxID, id),
		customopts.WithAnnotation(annotations.SandboxLogDir, config.GetLogDirectory()),
	)

	return c.runtimeSpec(id, "", specOpts...)
}

// sandboxContainerSpecOpts generates OCI spec options for
// the sandbox container.
func (c *criService) sandboxContainerSpecOpts(config *runtime.PodSandboxConfig, imageConfig *imagespec.ImageConfig) ([]oci.SpecOpts, error) {
	var (
		securityContext = config.GetLinux().GetSecurityContext()
		specOpts        []oci.SpecOpts
	)
	seccompSpecOpts, err := c.generateSeccompSpecOpts(
		securityContext.GetSeccompProfilePath(),
		securityContext.GetPrivileged(),
		c.seccompEnabled())
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate seccomp spec opts")
	}
	if seccompSpecOpts != nil {
		specOpts = append(specOpts, seccompSpecOpts)
	}

	userstr, err := generateUserString(
		"",
		securityContext.GetRunAsUser(),
		securityContext.GetRunAsGroup(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate user string")
	}
	if userstr == "" {
		// Lastly, since no user override was passed via CRI try to set via OCI
		// Image
		userstr = imageConfig.User
	}
	if userstr != "" {
		specOpts = append(specOpts, oci.WithUser(userstr))
	}
	return specOpts, nil
}

// setupSandboxFiles sets up necessary sandbox files including /dev/shm, /etc/hosts,
// /etc/resolv.conf and /etc/hostname.
func (c *criService) setupSandboxFiles(id string, config *runtime.PodSandboxConfig) error {
	sandboxEtcHostname := c.getSandboxHostname(id)
	hostname := config.GetHostname()
	if hostname == "" {
		var err error
		hostname, err = c.os.Hostname()
		if err != nil {
			return errors.Wrap(err, "failed to get hostname")
		}
	}
	if err := c.os.WriteFile(sandboxEtcHostname, []byte(hostname+"\n"), 0644); err != nil {
		return errors.Wrapf(err, "failed to write hostname to %q", sandboxEtcHostname)
	}

	// TODO(random-liu): Consider whether we should maintain /etc/hosts and /etc/resolv.conf in kubelet.
	sandboxEtcHosts := c.getSandboxHosts(id)
	if err := c.os.CopyFile(etcHosts, sandboxEtcHosts, 0644); err != nil {
		return errors.Wrapf(err, "failed to generate sandbox hosts file %q", sandboxEtcHosts)
	}

	// Set DNS options. Maintain a resolv.conf for the sandbox.
	var err error
	resolvContent := ""
	if dnsConfig := config.GetDnsConfig(); dnsConfig != nil {
		resolvContent, err = parseDNSOptions(dnsConfig.Servers, dnsConfig.Searches, dnsConfig.Options)
		if err != nil {
			return errors.Wrapf(err, "failed to parse sandbox DNSConfig %+v", dnsConfig)
		}
	}
	resolvPath := c.getResolvPath(id)
	if resolvContent == "" {
		// copy host's resolv.conf to resolvPath
		err = c.os.CopyFile(resolvConfPath, resolvPath, 0644)
		if err != nil {
			return errors.Wrapf(err, "failed to copy host's resolv.conf to %q", resolvPath)
		}
	} else {
		err = c.os.WriteFile(resolvPath, []byte(resolvContent), 0644)
		if err != nil {
			return errors.Wrapf(err, "failed to write resolv content to %q", resolvPath)
		}
	}

	// Setup sandbox /dev/shm.
	if config.GetLinux().GetSecurityContext().GetNamespaceOptions().GetIpc() == runtime.NamespaceMode_NODE {
		if _, err := c.os.Stat(devShm); err != nil {
			return errors.Wrapf(err, "host %q is not available for host ipc", devShm)
		}
	} else {
		sandboxDevShm := c.getSandboxDevShm(id)
		if err := c.os.MkdirAll(sandboxDevShm, 0700); err != nil {
			return errors.Wrap(err, "failed to create sandbox shm")
		}
		shmproperty := fmt.Sprintf("mode=1777,size=%d", defaultShmSize)
		if err := c.os.(osinterface.UNIX).Mount("shm", sandboxDevShm, "tmpfs", uintptr(unix.MS_NOEXEC|unix.MS_NOSUID|unix.MS_NODEV), shmproperty); err != nil {
			return errors.Wrap(err, "failed to mount sandbox shm")
		}
	}

	return nil
}

// parseDNSOptions parse DNS options into resolv.conf format content,
// if none option is specified, will return empty with no error.
func parseDNSOptions(servers, searches, options []string) (string, error) {
	resolvContent := ""

	if len(searches) > maxDNSSearches {
		return "", errors.Errorf("DNSOption.Searches has more than %d domains", maxDNSSearches)
	}

	if len(searches) > 0 {
		resolvContent += fmt.Sprintf("search %s\n", strings.Join(searches, " "))
	}

	if len(servers) > 0 {
		resolvContent += fmt.Sprintf("nameserver %s\n", strings.Join(servers, "\nnameserver "))
	}

	if len(options) > 0 {
		resolvContent += fmt.Sprintf("options %s\n", strings.Join(options, " "))
	}

	return resolvContent, nil
}

// cleanupSandboxFiles unmount some sandbox files, we rely on the removal of sandbox root directory to
// remove these files. Unmount should *NOT* return error if the mount point is already unmounted.
func (c *criService) cleanupSandboxFiles(id string, config *runtime.PodSandboxConfig) error {
	if config.GetLinux().GetSecurityContext().GetNamespaceOptions().GetIpc() != runtime.NamespaceMode_NODE {
		path, err := c.os.FollowSymlinkInScope(c.getSandboxDevShm(id), "/")
		if err != nil {
			return errors.Wrap(err, "failed to follow symlink")
		}
		if err := c.os.(osinterface.UNIX).Unmount(path); err != nil && !os.IsNotExist(err) {
			return errors.Wrapf(err, "failed to unmount %q", path)
		}
	}
	return nil
}

// taskOpts generates task options for a (sandbox) container.
func (c *criService) taskOpts(runtimeType string) []containerd.NewTaskOpts {
	// TODO(random-liu): Remove this after shim v1 is deprecated.
	var taskOpts []containerd.NewTaskOpts
	if c.config.NoPivot && (runtimeType == plugin.RuntimeRuncV1 || runtimeType == plugin.RuntimeRuncV2) {
		taskOpts = append(taskOpts, containerd.WithNoPivotRoot)
	}
	return taskOpts
}
