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
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	sandboxstore "github.com/containerd/cri/pkg/store/sandbox"
)

// ListPodSandbox returns a list of Sandbox.
func (c *criService) ListPodSandbox(ctx context.Context, r *runtime.ListPodSandboxRequest) (*runtime.ListPodSandboxResponse, error) {
	// List all sandboxes from store.
	sandboxesInStore := c.sandboxStore.List()
	var sandboxes []*runtime.PodSandbox
	for _, sandboxInStore := range sandboxesInStore {
		sandboxes = append(sandboxes, toCRISandbox(
			sandboxInStore.Metadata,
			sandboxInStore.Status.Get(),
		))
	}

	sandboxes = c.filterCRISandboxes(sandboxes, r.GetFilter())
	return &runtime.ListPodSandboxResponse{Items: sandboxes}, nil
}

// toCRISandbox converts sandbox metadata into CRI pod sandbox.
func toCRISandbox(meta sandboxstore.Metadata, status sandboxstore.Status) *runtime.PodSandbox {
	// Set sandbox state to NOTREADY by default.
	state := runtime.PodSandboxState_SANDBOX_NOTREADY
	if status.State == sandboxstore.StateReady {
		state = runtime.PodSandboxState_SANDBOX_READY
	}
	return &runtime.PodSandbox{
		Id:          meta.ID,
		Metadata:    meta.Config.GetMetadata(),
		State:       state,
		CreatedAt:   status.CreatedAt.UnixNano(),
		Labels:      meta.Config.GetLabels(),
		Annotations: meta.Config.GetAnnotations(),
	}
}

func (c *criService) normalizePodSandboxFilter(filter *runtime.PodSandboxFilter) {
	if sb, err := c.sandboxStore.Get(filter.GetId()); err == nil {
		filter.Id = sb.ID
	}
}

// filterCRISandboxes filters CRISandboxes.
func (c *criService) filterCRISandboxes(sandboxes []*runtime.PodSandbox, filter *runtime.PodSandboxFilter) []*runtime.PodSandbox {
	if filter == nil {
		return sandboxes
	}

	c.normalizePodSandboxFilter(filter)
	filtered := []*runtime.PodSandbox{}
	for _, s := range sandboxes {
		// Filter by id
		if filter.GetId() != "" && filter.GetId() != s.Id {
			continue
		}
		// Filter by state
		if filter.GetState() != nil && filter.GetState().GetState() != s.State {
			continue
		}
		// Filter by label
		if filter.GetLabelSelector() != nil {
			match := true
			for k, v := range filter.GetLabelSelector() {
				got, ok := s.Labels[k]
				if !ok || got != v {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		filtered = append(filtered, s)
	}

	return filtered
}
