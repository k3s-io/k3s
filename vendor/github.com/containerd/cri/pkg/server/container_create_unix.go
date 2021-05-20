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
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/containerd/cgroups"
	"github.com/containerd/containerd/contrib/apparmor"
	"github.com/containerd/containerd/contrib/seccomp"
	"github.com/containerd/containerd/oci"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/pkg/errors"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/annotations"
	"github.com/containerd/cri/pkg/config"
	customopts "github.com/containerd/cri/pkg/containerd/opts"
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

// containerMounts sets up necessary container system file mounts
// including /dev/shm, /etc/hosts and /etc/resolv.conf.
func (c *criService) containerMounts(sandboxID string, config *runtime.ContainerConfig) []*runtime.Mount {
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
			ContainerPath:  devShm,
			HostPath:       sandboxDevShm,
			Readonly:       false,
			SelinuxRelabel: sandboxDevShm != devShm,
		})
	}
	return mounts
}

func (c *criService) containerSpec(id string, sandboxID string, sandboxPid uint32, netNSPath string, containerName string,
	config *runtime.ContainerConfig, sandboxConfig *runtime.PodSandboxConfig, imageConfig *imagespec.ImageConfig,
	extraMounts []*runtime.Mount, ociRuntime config.Runtime) (_ *runtimespec.Spec, retErr error) {

	specOpts := []oci.SpecOpts{
		customopts.WithoutRunMount,
		customopts.WithoutDefaultSecuritySettings,
		customopts.WithRelativeRoot(relativeRootfsPath),
		customopts.WithProcessArgs(config, imageConfig),
		oci.WithDefaultPathEnv,
		// this will be set based on the security context below
		oci.WithNewPrivileges,
	}
	if config.GetWorkingDir() != "" {
		specOpts = append(specOpts, oci.WithProcessCwd(config.GetWorkingDir()))
	} else if imageConfig.WorkingDir != "" {
		specOpts = append(specOpts, oci.WithProcessCwd(imageConfig.WorkingDir))
	}

	if config.GetTty() {
		specOpts = append(specOpts, oci.WithTTY)
	}

	// Add HOSTNAME env.
	var (
		err      error
		hostname = sandboxConfig.GetHostname()
	)
	if hostname == "" {
		if hostname, err = c.os.Hostname(); err != nil {
			return nil, err
		}
	}
	specOpts = append(specOpts, oci.WithEnv([]string{hostnameEnv + "=" + hostname}))

	// Apply envs from image config first, so that envs from container config
	// can override them.
	env := append([]string{}, imageConfig.Env...)
	for _, e := range config.GetEnvs() {
		env = append(env, e.GetKey()+"="+e.GetValue())
	}
	specOpts = append(specOpts, oci.WithEnv(env))

	securityContext := config.GetLinux().GetSecurityContext()
	labelOptions, err := toLabel(securityContext.GetSelinuxOptions())
	if err != nil {
		return nil, err
	}
	if len(labelOptions) == 0 {
		// Use pod level SELinux config
		if sandbox, err := c.sandboxStore.Get(sandboxID); err == nil {
			labelOptions, err = selinux.DupSecOpt(sandbox.ProcessLabel)
			if err != nil {
				return nil, err
			}
		}
	}

	processLabel, mountLabel, err := label.InitLabels(labelOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init selinux options %+v", securityContext.GetSelinuxOptions())
	}
	defer func() {
		if retErr != nil {
			_ = label.ReleaseLabel(processLabel)
		}
	}()

	specOpts = append(specOpts, customopts.WithMounts(c.os, config, extraMounts, mountLabel))

	if !c.config.DisableProcMount {
		// Apply masked paths if specified.
		// If the container is privileged, this will be cleared later on.
		if maskedPaths := securityContext.GetMaskedPaths(); maskedPaths != nil {
			specOpts = append(specOpts, oci.WithMaskedPaths(maskedPaths))
		}

		// Apply readonly paths if specified.
		// If the container is privileged, this will be cleared later on.
		if readonlyPaths := securityContext.GetReadonlyPaths(); readonlyPaths != nil {
			specOpts = append(specOpts, oci.WithReadonlyPaths(readonlyPaths))
		}
	}

	if securityContext.GetPrivileged() {
		if !sandboxConfig.GetLinux().GetSecurityContext().GetPrivileged() {
			return nil, errors.New("no privileged container allowed in sandbox")
		}
		specOpts = append(specOpts, oci.WithPrivileged)
		if !ociRuntime.PrivilegedWithoutHostDevices {
			specOpts = append(specOpts, oci.WithHostDevices, oci.WithAllDevicesAllowed)
		} else {
			// add requested devices by the config as host devices are not automatically added
			specOpts = append(specOpts, customopts.WithDevices(c.os, config), customopts.WithCapabilities(securityContext, c.allCaps))
		}
	} else { // not privileged
		specOpts = append(specOpts, customopts.WithDevices(c.os, config), customopts.WithCapabilities(securityContext, c.allCaps))
	}

	// Clear all ambient capabilities. The implication of non-root + caps
	// is not clearly defined in Kubernetes.
	// See https://github.com/kubernetes/kubernetes/issues/56374
	// Keep docker's behavior for now.
	specOpts = append(specOpts,
		customopts.WithoutAmbientCaps,
		customopts.WithSelinuxLabels(processLabel, mountLabel),
	)

	// TODO: Figure out whether we should set no new privilege for sandbox container by default
	if securityContext.GetNoNewPrivs() {
		specOpts = append(specOpts, oci.WithNoNewPrivileges)
	}
	// TODO(random-liu): [P1] Set selinux options (privileged or not).
	if securityContext.GetReadonlyRootfs() {
		specOpts = append(specOpts, oci.WithRootFSReadonly())
	}

	if c.config.DisableCgroup {
		specOpts = append(specOpts, customopts.WithDisabledCgroups)
	} else {
		specOpts = append(specOpts, customopts.WithResources(config.GetLinux().GetResources(), c.config.TolerateMissingHugetlbController, c.config.DisableHugetlbController))
		if sandboxConfig.GetLinux().GetCgroupParent() != "" {
			cgroupsPath := getCgroupsPath(sandboxConfig.GetLinux().GetCgroupParent(), id)
			specOpts = append(specOpts, oci.WithCgroup(cgroupsPath))
		}
	}

	supplementalGroups := securityContext.GetSupplementalGroups()

	for pKey, pValue := range getPassthroughAnnotations(sandboxConfig.Annotations,
		ociRuntime.PodAnnotations) {
		specOpts = append(specOpts, customopts.WithAnnotation(pKey, pValue))
	}

	for pKey, pValue := range getPassthroughAnnotations(config.Annotations,
		ociRuntime.ContainerAnnotations) {
		specOpts = append(specOpts, customopts.WithAnnotation(pKey, pValue))
	}

	specOpts = append(specOpts,
		customopts.WithOOMScoreAdj(config, c.config.RestrictOOMScoreAdj),
		customopts.WithPodNamespaces(securityContext, sandboxPid),
		customopts.WithSupplementalGroups(supplementalGroups),
		customopts.WithAnnotation(annotations.ContainerType, annotations.ContainerTypeContainer),
		customopts.WithAnnotation(annotations.SandboxID, sandboxID),
		customopts.WithAnnotation(annotations.ContainerName, containerName),
	)
	// cgroupns is used for hiding /sys/fs/cgroup from containers.
	// For compatibility, cgroupns is not used when running in cgroup v1 mode or in privileged.
	// https://github.com/containers/libpod/issues/4363
	// https://github.com/kubernetes/enhancements/blob/0e409b47497e398b369c281074485c8de129694f/keps/sig-node/20191118-cgroups-v2.md#cgroup-namespace
	if cgroups.Mode() == cgroups.Unified && !securityContext.GetPrivileged() {
		specOpts = append(specOpts, oci.WithLinuxNamespace(
			runtimespec.LinuxNamespace{
				Type: runtimespec.CgroupNamespace,
			}))
	}
	return c.runtimeSpec(id, ociRuntime.BaseRuntimeSpec, specOpts...)
}

func (c *criService) containerSpecOpts(config *runtime.ContainerConfig, imageConfig *imagespec.ImageConfig) ([]oci.SpecOpts, error) {
	var specOpts []oci.SpecOpts
	securityContext := config.GetLinux().GetSecurityContext()
	// Set container username. This could only be done by containerd, because it needs
	// access to the container rootfs. Pass user name to containerd, and let it overwrite
	// the spec for us.
	userstr, err := generateUserString(
		securityContext.GetRunAsUsername(),
		securityContext.GetRunAsUser(),
		securityContext.GetRunAsGroup())
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
		c.apparmorEnabled())
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate apparmor spec opts")
	}
	if apparmorSpecOpts != nil {
		specOpts = append(specOpts, apparmorSpecOpts)
	}

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
	return specOpts, nil
}

// generateSeccompSpecOpts generates containerd SpecOpts for seccomp.
func (c *criService) generateSeccompSpecOpts(seccompProf string, privileged, seccompEnabled bool) (oci.SpecOpts, error) {
	if privileged {
		// Do not set seccomp profile when container is privileged
		return nil, nil
	}
	if seccompProf == "" {
		seccompProf = c.config.UnsetSeccompProfile
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
	// Based on kubernetes#51746, default apparmor profile should be applied
	// for when apparmor is not specified.
	case runtimeDefault, "":
		if privileged {
			// Do not set apparmor profile when container is privileged
			return nil, nil
		}
		// TODO (mikebrow): delete created apparmor default profile
		return apparmor.WithDefaultProfile(appArmorDefaultProfileName), nil
	case unconfinedProfile:
		return nil, nil
	default:
		// Require and Trim default profile name prefix
		if !strings.HasPrefix(apparmorProf, profileNamePrefix) {
			return nil, errors.Errorf("invalid apparmor profile %q", apparmorProf)
		}
		appArmorProfile := strings.TrimPrefix(apparmorProf, profileNamePrefix)
		if profileExists, err := appArmorProfileExists(appArmorProfile); !profileExists {
			if err != nil {
				return nil, errors.Wrap(err, "failed to generate apparmor spec opts")
			}
			return nil, errors.Errorf("apparmor profile not found %s", appArmorProfile)
		}
		return apparmor.WithProfile(appArmorProfile), nil
	}
}

// appArmorProfileExists scans apparmor/profiles for the requested profile
func appArmorProfileExists(profile string) (bool, error) {
	if profile == "" {
		return false, errors.New("nil apparmor profile is not supported")
	}
	profiles, err := os.Open("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		return false, err
	}
	defer profiles.Close()

	rbuff := bufio.NewReader(profiles)
	for {
		line, err := rbuff.ReadString('\n')
		switch err {
		case nil:
			if strings.HasPrefix(line, profile+" (") {
				return true, nil
			}
		case io.EOF:
			return false, nil
		default:
			return false, err
		}
	}
}

// generateUserString generates valid user string based on OCI Image Spec
// v1.0.0.
//
// CRI defines that the following combinations are valid:
//
// (none) -> ""
// username -> username
// username, uid -> username
// username, uid, gid -> username:gid
// username, gid -> username:gid
// uid -> uid
// uid, gid -> uid:gid
// gid -> error
//
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
