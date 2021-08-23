// +build windows

/*
Copyright 2018 The Kubernetes Authors.

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

package kuberuntime

import (
	"fmt"
	"runtime"

	v1 "k8s.io/api/core/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/features"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/securitycontext"
)

// applyPlatformSpecificContainerConfig applies platform specific configurations to runtimeapi.ContainerConfig.
func (m *kubeGenericRuntimeManager) applyPlatformSpecificContainerConfig(config *runtimeapi.ContainerConfig, container *v1.Container, pod *v1.Pod, uid *int64, username string, _ *kubecontainer.ContainerID) error {
	windowsConfig, err := m.generateWindowsContainerConfig(container, pod, uid, username)
	if err != nil {
		return err
	}
	config.Windows = windowsConfig

	return nil
}

// generateWindowsContainerConfig generates windows container config for kubelet runtime v1.
// Refer https://github.com/kubernetes/community/blob/master/contributors/design-proposals/node/cri-windows.md.
func (m *kubeGenericRuntimeManager) generateWindowsContainerConfig(container *v1.Container, pod *v1.Pod, uid *int64, username string) (*runtimeapi.WindowsContainerConfig, error) {
	wc := &runtimeapi.WindowsContainerConfig{
		Resources:       &runtimeapi.WindowsContainerResources{},
		SecurityContext: &runtimeapi.WindowsContainerSecurityContext{},
	}

	cpuLimit := container.Resources.Limits.Cpu()
	if !cpuLimit.IsZero() {
		// Note that sysinfo.NumCPU() is limited to 64 CPUs on Windows due to Processor Groups,
		// as only 64 processors are available for execution by a given process. This causes
		// some oddities on systems with more than 64 processors.
		// Refer https://msdn.microsoft.com/en-us/library/windows/desktop/dd405503(v=vs.85).aspx.

		// Since Kubernetes doesn't have any notion of weight in the Pod/Container API, only limits/reserves, then applying CpuMaximum only
		// will better follow the intent of the user. At one point CpuWeights were set, but this prevented limits from having any effect.

		// There are 3 parts to how this works:
		// Part one - Windows kernel
		//   cpuMaximum is documented at https://docs.microsoft.com/en-us/virtualization/windowscontainers/manage-containers/resource-controls
		//   the range and how it relates to number of CPUs is at https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_cpu_rate_control_information
		//   For process isolation, these are applied to the job object setting JOB_OBJECT_CPU_RATE_CONTROL_ENABLE, which can be set to either
		//   JOB_OBJECT_CPU_RATE_CONTROL_WEIGHT_BASED or JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP. This is why the settings are mutually exclusive.
		// Part two - Docker (doc: https://docs.docker.com/engine/api/v1.30)
		//   If both CpuWeight and CpuMaximum are passed to Docker, then it sets
		//   JOB_OBJECT_CPU_RATE_CONTROL_ENABLE = JOB_OBJECT_CPU_RATE_CONTROL_WEIGHT_BASED ignoring CpuMaximum.
		//   Option a: Set HostConfig.CpuPercent. The units are whole percent of the total CPU capacity of the system, meaning the resolution
		//      is different based on the number of cores.
		//   Option b: Set HostConfig.NanoCpus integer <int64> - CPU quota in units of 10e-9 CPUs. Moby scales this to the Windows job object
		//      resolution of 1-10000, so it's higher resolution than option a.
		//      src: https://github.com/moby/moby/blob/10866714412aea1bb587d1ad14b2ce1ba4cf4308/daemon/oci_windows.go#L426
		// Part three - CRI & ContainerD's implementation
		//   The kubelet sets these directly on CGroups in Linux, but needs to pass them across CRI on Windows.
		//   There is an existing cpu_maximum field, with a range of percent * 100, so 1-10000. This is different from Docker, but consistent with OCI
		//   https://github.com/kubernetes/kubernetes/blob/56d1c3b96d0a544130a82caad33dd57629b8a7f8/staging/src/k8s.io/cri-api/pkg/apis/runtime/v1alpha2/api.proto#L681-L682
		//   https://github.com/opencontainers/runtime-spec/blob/ad53dcdc39f1f7f7472b10aa0a45648fe4865496/config-windows.md#cpu
		//   If both CpuWeight and CpuMaximum are set - ContainerD catches this invalid case and returns an error instead.

		cpuMaximum := 10000 * cpuLimit.MilliValue() / int64(runtime.NumCPU()) / 1000

		// ensure cpuMaximum is in range [1, 10000].
		if cpuMaximum < 1 {
			cpuMaximum = 1
		} else if cpuMaximum > 10000 {
			cpuMaximum = 10000
		}

		wc.Resources.CpuMaximum = cpuMaximum
	}

	// The processor resource controls are mutually exclusive on
	// Windows Server Containers, the order of precedence is
	// CPUCount first, then CPUMaximum.
	if wc.Resources.CpuCount > 0 {
		if wc.Resources.CpuMaximum > 0 {
			wc.Resources.CpuMaximum = 0
			klog.InfoS("Mutually exclusive options: CPUCount priority > CPUMaximum priority on Windows Server Containers. CPUMaximum should be ignored")
		}
	}

	memoryLimit := container.Resources.Limits.Memory().Value()
	if memoryLimit != 0 {
		wc.Resources.MemoryLimitInBytes = memoryLimit
	}

	// setup security context
	effectiveSc := securitycontext.DetermineEffectiveSecurityContext(pod, container)

	if username != "" {
		wc.SecurityContext.RunAsUsername = username
	}
	if effectiveSc.WindowsOptions != nil &&
		effectiveSc.WindowsOptions.GMSACredentialSpec != nil {
		wc.SecurityContext.CredentialSpec = *effectiveSc.WindowsOptions.GMSACredentialSpec
	}

	// override with Windows options if present
	if effectiveSc.WindowsOptions != nil && effectiveSc.WindowsOptions.RunAsUserName != nil {
		wc.SecurityContext.RunAsUsername = *effectiveSc.WindowsOptions.RunAsUserName
	}

	if securitycontext.HasWindowsHostProcessRequest(pod, container) {
		if !utilfeature.DefaultFeatureGate.Enabled(features.WindowsHostProcessContainers) {
			return nil, fmt.Errorf("pod contains HostProcess containers but feature 'WindowsHostProcessContainers' is not enabled")
		}
		wc.SecurityContext.HostProcess = true
	}

	return wc, nil
}
