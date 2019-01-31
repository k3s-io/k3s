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
	"encoding/json"

	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/store"
	containerstore "github.com/containerd/cri/pkg/store/container"
)

// ContainerStatus inspects the container and returns the status.
func (c *criService) ContainerStatus(ctx context.Context, r *runtime.ContainerStatusRequest) (*runtime.ContainerStatusResponse, error) {
	container, err := c.containerStore.Get(r.GetContainerId())
	if err != nil {
		return nil, errors.Wrapf(err, "an error occurred when try to find container %q", r.GetContainerId())
	}

	// TODO(random-liu): Clean up the following logic in CRI.
	// Current assumption:
	// * ImageSpec in container config is image ID.
	// * ImageSpec in container status is image tag.
	// * ImageRef in container status is repo digest.
	spec := container.Config.GetImage()
	imageRef := container.ImageRef
	image, err := c.imageStore.Get(imageRef)
	if err != nil {
		if err != store.ErrNotExist {
			return nil, errors.Wrapf(err, "failed to get image %q", imageRef)
		}
	} else {
		repoTags, repoDigests := parseImageReferences(image.References)
		if len(repoTags) > 0 {
			// Based on current behavior of dockershim, this field should be
			// image tag.
			spec = &runtime.ImageSpec{Image: repoTags[0]}
		}
		if len(repoDigests) > 0 {
			// Based on the CRI definition, this field will be consumed by user.
			imageRef = repoDigests[0]
		}
	}
	status := toCRIContainerStatus(container, spec, imageRef)
	info, err := toCRIContainerInfo(ctx, container, r.GetVerbose())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get verbose container info")
	}

	return &runtime.ContainerStatusResponse{
		Status: status,
		Info:   info,
	}, nil
}

// toCRIContainerStatus converts internal container object to CRI container status.
func toCRIContainerStatus(container containerstore.Container, spec *runtime.ImageSpec, imageRef string) *runtime.ContainerStatus {
	meta := container.Metadata
	status := container.Status.Get()
	reason := status.Reason
	if status.State() == runtime.ContainerState_CONTAINER_EXITED && reason == "" {
		if status.ExitCode == 0 {
			reason = completeExitReason
		} else {
			reason = errorExitReason
		}
	}

	return &runtime.ContainerStatus{
		Id:          meta.ID,
		Metadata:    meta.Config.GetMetadata(),
		State:       status.State(),
		CreatedAt:   status.CreatedAt,
		StartedAt:   status.StartedAt,
		FinishedAt:  status.FinishedAt,
		ExitCode:    status.ExitCode,
		Image:       spec,
		ImageRef:    imageRef,
		Reason:      reason,
		Message:     status.Message,
		Labels:      meta.Config.GetLabels(),
		Annotations: meta.Config.GetAnnotations(),
		Mounts:      meta.Config.GetMounts(),
		LogPath:     meta.LogPath,
	}
}

// ContainerInfo is extra information for a container.
type ContainerInfo struct {
	// TODO(random-liu): Add sandboxID in CRI container status.
	SandboxID      string                   `json:"sandboxID"`
	Pid            uint32                   `json:"pid"`
	Removing       bool                     `json:"removing"`
	SnapshotKey    string                   `json:"snapshotKey"`
	Snapshotter    string                   `json:"snapshotter"`
	RuntimeType    string                   `json:"runtimeType"`
	RuntimeOptions interface{}              `json:"runtimeOptions"`
	Config         *runtime.ContainerConfig `json:"config"`
	RuntimeSpec    *runtimespec.Spec        `json:"runtimeSpec"`
}

// toCRIContainerInfo converts internal container object information to CRI container status response info map.
func toCRIContainerInfo(ctx context.Context, container containerstore.Container, verbose bool) (map[string]string, error) {
	if !verbose {
		return nil, nil
	}

	meta := container.Metadata
	status := container.Status.Get()

	// TODO(random-liu): Change CRI status info to use array instead of map.
	ci := &ContainerInfo{
		SandboxID: container.SandboxID,
		Pid:       status.Pid,
		Removing:  status.Removing,
		Config:    meta.Config,
	}

	var err error
	ci.RuntimeSpec, err = container.Container.Spec(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get container runtime spec")
	}

	ctrInfo, err := container.Container.Info(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get container info")
	}
	ci.SnapshotKey = ctrInfo.SnapshotKey
	ci.Snapshotter = ctrInfo.Snapshotter

	runtimeOptions, err := getRuntimeOptions(ctrInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get runtime options")
	}
	ci.RuntimeType = ctrInfo.Runtime.Name
	ci.RuntimeOptions = runtimeOptions

	infoBytes, err := json.Marshal(ci)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal info %v", ci)
	}
	return map[string]string{
		"info": string(infoBytes),
	}, nil
}
