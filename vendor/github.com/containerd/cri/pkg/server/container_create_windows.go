// +build windows

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
	"github.com/containerd/containerd/oci"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/annotations"
	"github.com/containerd/cri/pkg/config"
	customopts "github.com/containerd/cri/pkg/containerd/opts"
)

// No container mounts for windows.
func (c *criService) containerMounts(sandboxID string, config *runtime.ContainerConfig) []*runtime.Mount {
	return nil
}

func (c *criService) containerSpec(id string, sandboxID string, sandboxPid uint32, netNSPath string, containerName string,
	config *runtime.ContainerConfig, sandboxConfig *runtime.PodSandboxConfig, imageConfig *imagespec.ImageConfig,
	extraMounts []*runtime.Mount, ociRuntime config.Runtime) (*runtimespec.Spec, error) {
	specOpts := []oci.SpecOpts{
		customopts.WithProcessArgs(config, imageConfig),
	}
	if config.GetWorkingDir() != "" {
		specOpts = append(specOpts, oci.WithProcessCwd(config.GetWorkingDir()))
	} else if imageConfig.WorkingDir != "" {
		specOpts = append(specOpts, oci.WithProcessCwd(imageConfig.WorkingDir))
	}

	if config.GetTty() {
		specOpts = append(specOpts, oci.WithTTY)
	}

	// Apply envs from image config first, so that envs from container config
	// can override them.
	env := imageConfig.Env
	for _, e := range config.GetEnvs() {
		env = append(env, e.GetKey()+"="+e.GetValue())
	}
	specOpts = append(specOpts, oci.WithEnv(env))

	specOpts = append(specOpts,
		// Clear the root location since hcsshim expects it.
		// NOTE: readonly rootfs doesn't work on windows.
		customopts.WithoutRoot,
		customopts.WithWindowsNetworkNamespace(netNSPath),
		oci.WithHostname(sandboxConfig.GetHostname()),
	)

	specOpts = append(specOpts, customopts.WithWindowsMounts(c.os, config, extraMounts))

	// Start with the image config user and override below if RunAsUsername is not "".
	username := imageConfig.User

	windowsConfig := config.GetWindows()
	if windowsConfig != nil {
		specOpts = append(specOpts, customopts.WithWindowsResources(windowsConfig.GetResources()))
		securityCtx := windowsConfig.GetSecurityContext()
		if securityCtx != nil {
			runAsUser := securityCtx.GetRunAsUsername()
			if runAsUser != "" {
				username = runAsUser
			}
			cs := securityCtx.GetCredentialSpec()
			if cs != "" {
				specOpts = append(specOpts, customopts.WithWindowsCredentialSpec(cs))
			}
		}
	}

	// There really isn't a good Windows way to verify that the username is available in the
	// image as early as here like there is for Linux. Later on in the stack hcsshim
	// will handle the behavior of erroring out if the user isn't available in the image
	// when trying to run the init process.
	specOpts = append(specOpts, oci.WithUser(username))

	for pKey, pValue := range getPassthroughAnnotations(sandboxConfig.Annotations,
		ociRuntime.PodAnnotations) {
		specOpts = append(specOpts, customopts.WithAnnotation(pKey, pValue))
	}

	for pKey, pValue := range getPassthroughAnnotations(config.Annotations,
		ociRuntime.ContainerAnnotations) {
		specOpts = append(specOpts, customopts.WithAnnotation(pKey, pValue))
	}

	specOpts = append(specOpts,
		customopts.WithAnnotation(annotations.ContainerType, annotations.ContainerTypeContainer),
		customopts.WithAnnotation(annotations.SandboxID, sandboxID),
		customopts.WithAnnotation(annotations.ContainerName, containerName),
	)
	return c.runtimeSpec(id, ociRuntime.BaseRuntimeSpec, specOpts...)
}

// No extra spec options needed for windows.
func (c *criService) containerSpecOpts(config *runtime.ContainerConfig, imageConfig *imagespec.ImageConfig) ([]oci.SpecOpts, error) {
	return nil, nil
}
