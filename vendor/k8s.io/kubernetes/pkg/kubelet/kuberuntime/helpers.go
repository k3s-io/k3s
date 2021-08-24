/*
Copyright 2016 The Kubernetes Authors.

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
	"path/filepath"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog/v2"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
)

type podsByID []*kubecontainer.Pod

func (b podsByID) Len() int           { return len(b) }
func (b podsByID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b podsByID) Less(i, j int) bool { return b[i].ID < b[j].ID }

type containersByID []*kubecontainer.Container

func (b containersByID) Len() int           { return len(b) }
func (b containersByID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b containersByID) Less(i, j int) bool { return b[i].ID.ID < b[j].ID.ID }

// Newest first.
type podSandboxByCreated []*runtimeapi.PodSandbox

func (p podSandboxByCreated) Len() int           { return len(p) }
func (p podSandboxByCreated) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p podSandboxByCreated) Less(i, j int) bool { return p[i].CreatedAt > p[j].CreatedAt }

type containerStatusByCreated []*kubecontainer.Status

func (c containerStatusByCreated) Len() int           { return len(c) }
func (c containerStatusByCreated) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c containerStatusByCreated) Less(i, j int) bool { return c[i].CreatedAt.After(c[j].CreatedAt) }

// toKubeContainerState converts runtimeapi.ContainerState to kubecontainer.State.
func toKubeContainerState(state runtimeapi.ContainerState) kubecontainer.State {
	switch state {
	case runtimeapi.ContainerState_CONTAINER_CREATED:
		return kubecontainer.ContainerStateCreated
	case runtimeapi.ContainerState_CONTAINER_RUNNING:
		return kubecontainer.ContainerStateRunning
	case runtimeapi.ContainerState_CONTAINER_EXITED:
		return kubecontainer.ContainerStateExited
	case runtimeapi.ContainerState_CONTAINER_UNKNOWN:
		return kubecontainer.ContainerStateUnknown
	}

	return kubecontainer.ContainerStateUnknown
}

// toRuntimeProtocol converts v1.Protocol to runtimeapi.Protocol.
func toRuntimeProtocol(protocol v1.Protocol) runtimeapi.Protocol {
	switch protocol {
	case v1.ProtocolTCP:
		return runtimeapi.Protocol_TCP
	case v1.ProtocolUDP:
		return runtimeapi.Protocol_UDP
	case v1.ProtocolSCTP:
		return runtimeapi.Protocol_SCTP
	}

	klog.InfoS("Unknown protocol, defaulting to TCP", "protocol", protocol)
	return runtimeapi.Protocol_TCP
}

// toKubeContainer converts runtimeapi.Container to kubecontainer.Container.
func (m *kubeGenericRuntimeManager) toKubeContainer(c *runtimeapi.Container) (*kubecontainer.Container, error) {
	if c == nil || c.Id == "" || c.Image == nil {
		return nil, fmt.Errorf("unable to convert a nil pointer to a runtime container")
	}

	annotatedInfo := getContainerInfoFromAnnotations(c.Annotations)
	return &kubecontainer.Container{
		ID:      kubecontainer.ContainerID{Type: m.runtimeName, ID: c.Id},
		Name:    c.GetMetadata().GetName(),
		ImageID: c.ImageRef,
		Image:   c.Image.Image,
		Hash:    annotatedInfo.Hash,
		State:   toKubeContainerState(c.State),
	}, nil
}

// sandboxToKubeContainer converts runtimeapi.PodSandbox to kubecontainer.Container.
// This is only needed because we need to return sandboxes as if they were
// kubecontainer.Containers to avoid substantial changes to PLEG.
// TODO: Remove this once it becomes obsolete.
func (m *kubeGenericRuntimeManager) sandboxToKubeContainer(s *runtimeapi.PodSandbox) (*kubecontainer.Container, error) {
	if s == nil || s.Id == "" {
		return nil, fmt.Errorf("unable to convert a nil pointer to a runtime container")
	}

	return &kubecontainer.Container{
		ID:    kubecontainer.ContainerID{Type: m.runtimeName, ID: s.Id},
		State: kubecontainer.SandboxToContainerState(s.State),
	}, nil
}

// getImageUser gets uid or user name that will run the command(s) from image. The function
// guarantees that only one of them is set.
func (m *kubeGenericRuntimeManager) getImageUser(image string) (*int64, string, error) {
	imageStatus, err := m.imageService.ImageStatus(&runtimeapi.ImageSpec{Image: image})
	if err != nil {
		return nil, "", err
	}

	if imageStatus != nil {
		if imageStatus.Uid != nil {
			return &imageStatus.GetUid().Value, "", nil
		}

		if imageStatus.Username != "" {
			return nil, imageStatus.Username, nil
		}
	}

	// If non of them is set, treat it as root.
	return new(int64), "", nil
}

// isInitContainerFailed returns true if container has exited and exitcode is not zero
// or is in unknown state.
func isInitContainerFailed(status *kubecontainer.Status) bool {
	if status.State == kubecontainer.ContainerStateExited && status.ExitCode != 0 {
		return true
	}

	if status.State == kubecontainer.ContainerStateUnknown {
		return true
	}

	return false
}

// getStableKey generates a key (string) to uniquely identify a
// (pod, container) tuple. The key should include the content of the
// container, so that any change to the container generates a new key.
func getStableKey(pod *v1.Pod, container *v1.Container) string {
	hash := strconv.FormatUint(kubecontainer.HashContainer(container), 16)
	return fmt.Sprintf("%s_%s_%s_%s_%s", pod.Name, pod.Namespace, string(pod.UID), container.Name, hash)
}

// logPathDelimiter is the delimiter used in the log path.
const logPathDelimiter = "_"

// buildContainerLogsPath builds log path for container relative to pod logs directory.
func buildContainerLogsPath(containerName string, restartCount int) string {
	return filepath.Join(containerName, fmt.Sprintf("%d.log", restartCount))
}

// BuildContainerLogsDirectory builds absolute log directory path for a container in pod.
func BuildContainerLogsDirectory(podNamespace, podName string, podUID types.UID, containerName string) string {
	return filepath.Join(BuildPodLogsDirectory(podNamespace, podName, podUID), containerName)
}

// BuildPodLogsDirectory builds absolute log directory path for a pod sandbox.
func BuildPodLogsDirectory(podNamespace, podName string, podUID types.UID) string {
	return filepath.Join(podLogsRootDirectory, strings.Join([]string{podNamespace, podName,
		string(podUID)}, logPathDelimiter))
}

// parsePodUIDFromLogsDirectory parses pod logs directory name and returns the pod UID.
// It supports both the old pod log directory /var/log/pods/UID, and the new pod log
// directory /var/log/pods/NAMESPACE_NAME_UID.
func parsePodUIDFromLogsDirectory(name string) types.UID {
	parts := strings.Split(name, logPathDelimiter)
	return types.UID(parts[len(parts)-1])
}

// toKubeRuntimeStatus converts the runtimeapi.RuntimeStatus to kubecontainer.RuntimeStatus.
func toKubeRuntimeStatus(status *runtimeapi.RuntimeStatus) *kubecontainer.RuntimeStatus {
	conditions := []kubecontainer.RuntimeCondition{}
	for _, c := range status.GetConditions() {
		conditions = append(conditions, kubecontainer.RuntimeCondition{
			Type:    kubecontainer.RuntimeConditionType(c.Type),
			Status:  c.Status,
			Reason:  c.Reason,
			Message: c.Message,
		})
	}
	return &kubecontainer.RuntimeStatus{Conditions: conditions}
}

func fieldProfile(scmp *v1.SeccompProfile, profileRootPath string, fallbackToRuntimeDefault bool) string {
	if scmp == nil {
		if fallbackToRuntimeDefault {
			return v1.SeccompProfileRuntimeDefault
		}
		return ""
	}
	if scmp.Type == v1.SeccompProfileTypeRuntimeDefault {
		return v1.SeccompProfileRuntimeDefault
	}
	if scmp.Type == v1.SeccompProfileTypeLocalhost && scmp.LocalhostProfile != nil && len(*scmp.LocalhostProfile) > 0 {
		fname := filepath.Join(profileRootPath, *scmp.LocalhostProfile)
		return v1.SeccompLocalhostProfileNamePrefix + fname
	}
	if scmp.Type == v1.SeccompProfileTypeUnconfined {
		return v1.SeccompProfileNameUnconfined
	}

	if fallbackToRuntimeDefault {
		return v1.SeccompProfileRuntimeDefault
	}
	return ""
}

func annotationProfile(profile, profileRootPath string) string {
	if strings.HasPrefix(profile, v1.SeccompLocalhostProfileNamePrefix) {
		name := strings.TrimPrefix(profile, v1.SeccompLocalhostProfileNamePrefix)
		fname := filepath.Join(profileRootPath, filepath.FromSlash(name))
		return v1.SeccompLocalhostProfileNamePrefix + fname
	}
	return profile
}

func (m *kubeGenericRuntimeManager) getSeccompProfilePath(annotations map[string]string, containerName string,
	podSecContext *v1.PodSecurityContext, containerSecContext *v1.SecurityContext, fallbackToRuntimeDefault bool) string {
	// container fields are applied first
	if containerSecContext != nil && containerSecContext.SeccompProfile != nil {
		return fieldProfile(containerSecContext.SeccompProfile, m.seccompProfileRoot, fallbackToRuntimeDefault)
	}

	// if container field does not exist, try container annotation (deprecated)
	if containerName != "" {
		if profile, ok := annotations[v1.SeccompContainerAnnotationKeyPrefix+containerName]; ok {
			return annotationProfile(profile, m.seccompProfileRoot)
		}
	}

	// when container seccomp is not defined, try to apply from pod field
	if podSecContext != nil && podSecContext.SeccompProfile != nil {
		return fieldProfile(podSecContext.SeccompProfile, m.seccompProfileRoot, fallbackToRuntimeDefault)
	}

	// as last resort, try to apply pod annotation (deprecated)
	if profile, ok := annotations[v1.SeccompPodAnnotationKey]; ok {
		return annotationProfile(profile, m.seccompProfileRoot)
	}

	if fallbackToRuntimeDefault {
		return v1.SeccompProfileRuntimeDefault
	}

	return ""
}

func fieldSeccompProfile(scmp *v1.SeccompProfile, profileRootPath string, fallbackToRuntimeDefault bool) *runtimeapi.SecurityProfile {
	if scmp == nil {
		if fallbackToRuntimeDefault {
			return &runtimeapi.SecurityProfile{
				ProfileType: runtimeapi.SecurityProfile_RuntimeDefault,
			}
		}
		return &runtimeapi.SecurityProfile{
			ProfileType: runtimeapi.SecurityProfile_Unconfined,
		}
	}
	if scmp.Type == v1.SeccompProfileTypeRuntimeDefault {
		return &runtimeapi.SecurityProfile{
			ProfileType: runtimeapi.SecurityProfile_RuntimeDefault,
		}
	}
	if scmp.Type == v1.SeccompProfileTypeLocalhost && scmp.LocalhostProfile != nil && len(*scmp.LocalhostProfile) > 0 {
		fname := filepath.Join(profileRootPath, *scmp.LocalhostProfile)
		return &runtimeapi.SecurityProfile{
			ProfileType:  runtimeapi.SecurityProfile_Localhost,
			LocalhostRef: fname,
		}
	}
	return &runtimeapi.SecurityProfile{
		ProfileType: runtimeapi.SecurityProfile_Unconfined,
	}
}

func (m *kubeGenericRuntimeManager) getSeccompProfile(annotations map[string]string, containerName string,
	podSecContext *v1.PodSecurityContext, containerSecContext *v1.SecurityContext, fallbackToRuntimeDefault bool) *runtimeapi.SecurityProfile {
	// container fields are applied first
	if containerSecContext != nil && containerSecContext.SeccompProfile != nil {
		return fieldSeccompProfile(containerSecContext.SeccompProfile, m.seccompProfileRoot, fallbackToRuntimeDefault)
	}

	// when container seccomp is not defined, try to apply from pod field
	if podSecContext != nil && podSecContext.SeccompProfile != nil {
		return fieldSeccompProfile(podSecContext.SeccompProfile, m.seccompProfileRoot, fallbackToRuntimeDefault)
	}

	if fallbackToRuntimeDefault {
		return &runtimeapi.SecurityProfile{
			ProfileType: runtimeapi.SecurityProfile_RuntimeDefault,
		}
	}

	return &runtimeapi.SecurityProfile{
		ProfileType: runtimeapi.SecurityProfile_Unconfined,
	}
}

func ipcNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil && pod.Spec.HostIPC {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

func networkNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil && pod.Spec.HostNetwork {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

func pidNamespaceForPod(pod *v1.Pod) runtimeapi.NamespaceMode {
	if pod != nil {
		if pod.Spec.HostPID {
			return runtimeapi.NamespaceMode_NODE
		}
		if pod.Spec.ShareProcessNamespace != nil && *pod.Spec.ShareProcessNamespace {
			return runtimeapi.NamespaceMode_POD
		}
	}
	// Note that PID does not default to the zero value for v1.Pod
	return runtimeapi.NamespaceMode_CONTAINER
}

// namespacesForPod returns the runtimeapi.NamespaceOption for a given pod.
// An empty or nil pod can be used to get the namespace defaults for v1.Pod.
func namespacesForPod(pod *v1.Pod) *runtimeapi.NamespaceOption {
	return &runtimeapi.NamespaceOption{
		Ipc:     ipcNamespaceForPod(pod),
		Network: networkNamespaceForPod(pod),
		Pid:     pidNamespaceForPod(pod),
	}
}
