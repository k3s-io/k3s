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
	gocontext "context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/typeurl"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	containerstore "github.com/containerd/cri/pkg/store/container"
	"github.com/containerd/cri/pkg/util"
)

// UpdateContainerResources updates ContainerConfig of the container.
func (c *criService) UpdateContainerResources(ctx context.Context, r *runtime.UpdateContainerResourcesRequest) (retRes *runtime.UpdateContainerResourcesResponse, retErr error) {
	container, err := c.containerStore.Get(r.GetContainerId())
	if err != nil {
		return nil, errors.Wrap(err, "failed to find container")
	}
	// Update resources in status update transaction, so that:
	// 1) There won't be race condition with container start.
	// 2) There won't be concurrent resource update to the same container.
	if err := container.Status.Update(func(status containerstore.Status) (containerstore.Status, error) {
		return status, c.updateContainerResources(ctx, container, r.GetLinux(), status)
	}); err != nil {
		return nil, errors.Wrap(err, "failed to update resources")
	}
	return &runtime.UpdateContainerResourcesResponse{}, nil
}

func (c *criService) updateContainerResources(ctx context.Context,
	cntr containerstore.Container,
	resources *runtime.LinuxContainerResources,
	status containerstore.Status) (retErr error) {
	id := cntr.ID
	// Do not update the container when there is a removal in progress.
	if status.Removing {
		return errors.Errorf("container %q is in removing state", id)
	}

	// Update container spec. If the container is not started yet, updating
	// spec makes sure that the resource limits are correct when start;
	// if the container is already started, updating spec is still required,
	// the spec will become our source of truth for resource limits.
	oldSpec, err := cntr.Container.Spec(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get container spec")
	}
	newSpec, err := updateOCILinuxResource(oldSpec, resources)
	if err != nil {
		return errors.Wrap(err, "failed to update resource in spec")
	}

	if err := updateContainerSpec(ctx, cntr.Container, newSpec); err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			deferCtx, deferCancel := ctrdutil.DeferContext()
			defer deferCancel()
			// Reset spec on error.
			if err := updateContainerSpec(deferCtx, cntr.Container, oldSpec); err != nil {
				logrus.WithError(err).Errorf("Failed to update spec %+v for container %q", oldSpec, id)
			}
		}
	}()

	// If container is not running, only update spec is enough, new resource
	// limit will be applied when container start.
	if status.State() != runtime.ContainerState_CONTAINER_RUNNING {
		return nil
	}

	task, err := cntr.Container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			// Task exited already.
			return nil
		}
		return errors.Wrap(err, "failed to get task")
	}
	// newSpec.Linux won't be nil
	if err := task.Update(ctx, containerd.WithResources(newSpec.Linux.Resources)); err != nil {
		if errdefs.IsNotFound(err) {
			// Task exited already.
			return nil
		}
		return errors.Wrap(err, "failed to update resources")
	}
	return nil
}

// updateContainerSpec updates container spec.
func updateContainerSpec(ctx context.Context, cntr containerd.Container, spec *runtimespec.Spec) error {
	any, err := typeurl.MarshalAny(spec)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal spec %+v", spec)
	}
	if err := cntr.Update(ctx, func(ctx gocontext.Context, client *containerd.Client, c *containers.Container) error {
		c.Spec = any
		return nil
	}); err != nil {
		return errors.Wrap(err, "failed to update container spec")
	}
	return nil
}

// updateOCILinuxResource updates container resource limit.
func updateOCILinuxResource(spec *runtimespec.Spec, new *runtime.LinuxContainerResources) (*runtimespec.Spec, error) {
	// Copy to make sure old spec is not changed.
	var cloned runtimespec.Spec
	if err := util.DeepCopy(&cloned, spec); err != nil {
		return nil, errors.Wrap(err, "failed to deep copy")
	}
	g := newSpecGenerator(&cloned)

	if new.GetCpuPeriod() != 0 {
		g.SetLinuxResourcesCPUPeriod(uint64(new.GetCpuPeriod()))
	}
	if new.GetCpuQuota() != 0 {
		g.SetLinuxResourcesCPUQuota(new.GetCpuQuota())
	}
	if new.GetCpuShares() != 0 {
		g.SetLinuxResourcesCPUShares(uint64(new.GetCpuShares()))
	}
	if new.GetMemoryLimitInBytes() != 0 {
		g.SetLinuxResourcesMemoryLimit(new.GetMemoryLimitInBytes())
	}
	// OOMScore is not updatable.
	if new.GetCpusetCpus() != "" {
		g.SetLinuxResourcesCPUCpus(new.GetCpusetCpus())
	}
	if new.GetCpusetMems() != "" {
		g.SetLinuxResourcesCPUMems(new.GetCpusetMems())
	}

	return g.Config, nil
}
