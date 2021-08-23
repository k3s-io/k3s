/*
Copyright 2020 The Kubernetes Authors.

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

package memorymanager

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/cm/containermap"
	"k8s.io/kubernetes/pkg/kubelet/cm/memorymanager/state"
	"k8s.io/kubernetes/pkg/kubelet/cm/topologymanager"
	"k8s.io/kubernetes/pkg/kubelet/config"
	"k8s.io/kubernetes/pkg/kubelet/status"
)

type fakeManager struct {
	state state.State
}

func (m *fakeManager) Start(activePods ActivePodsFunc, sourcesReady config.SourcesReady, podStatusProvider status.PodStatusProvider, containerRuntime runtimeService, initialContainers containermap.ContainerMap) error {
	klog.InfoS("Start()")
	return nil
}

func (m *fakeManager) Policy() Policy {
	klog.InfoS("Policy()")
	return NewPolicyNone()
}

func (m *fakeManager) Allocate(pod *v1.Pod, container *v1.Container) error {
	klog.InfoS("Allocate", "pod", klog.KObj(pod), "containerName", container.Name)
	return nil
}

func (m *fakeManager) AddContainer(pod *v1.Pod, container *v1.Container, containerID string) {
	klog.InfoS("Add container", "pod", klog.KObj(pod), "containerName", container.Name, "containerID", containerID)
}

func (m *fakeManager) GetMemoryNUMANodes(pod *v1.Pod, container *v1.Container) sets.Int {
	klog.InfoS("Get MemoryNUMANodes", "pod", klog.KObj(pod), "containerName", container.Name)
	return nil
}

func (m *fakeManager) RemoveContainer(containerID string) error {
	klog.InfoS("RemoveContainer", "containerID", containerID)
	return nil
}

func (m *fakeManager) GetTopologyHints(pod *v1.Pod, container *v1.Container) map[string][]topologymanager.TopologyHint {
	klog.InfoS("Get Topology Hints", "pod", klog.KObj(pod), "containerName", container.Name)
	return map[string][]topologymanager.TopologyHint{}
}

func (m *fakeManager) GetPodTopologyHints(pod *v1.Pod) map[string][]topologymanager.TopologyHint {
	klog.InfoS("Get Pod Topology Hints", "pod", klog.KObj(pod))
	return map[string][]topologymanager.TopologyHint{}
}

func (m *fakeManager) State() state.Reader {
	return m.state
}

// GetAllocatableMemory returns the amount of allocatable memory for each NUMA node
func (m *fakeManager) GetAllocatableMemory() []state.Block {
	klog.InfoS("Get Allocatable Memory")
	return []state.Block{}
}

// GetMemory returns the memory allocated by a container from NUMA nodes
func (m *fakeManager) GetMemory(podUID, containerName string) []state.Block {
	klog.InfoS("Get Memory", "podUID", podUID, "containerName", containerName)
	return []state.Block{}
}

// NewFakeManager creates empty/fake memory manager
func NewFakeManager() Manager {
	return &fakeManager{
		state: state.NewMemoryState(),
	}
}
