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
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/docker/docker/pkg/system"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/log"
	"github.com/containerd/cri/pkg/store"
	containerstore "github.com/containerd/cri/pkg/store/container"
)

// RemoveContainer removes the container.
// TODO(random-liu): Forcibly stop container if it's running.
func (c *criService) RemoveContainer(ctx context.Context, r *runtime.RemoveContainerRequest) (_ *runtime.RemoveContainerResponse, retErr error) {
	container, err := c.containerStore.Get(r.GetContainerId())
	if err != nil {
		if err != store.ErrNotExist {
			return nil, errors.Wrapf(err, "an error occurred when try to find container %q", r.GetContainerId())
		}
		// Do not return error if container metadata doesn't exist.
		log.Tracef("RemoveContainer called for container %q that does not exist", r.GetContainerId())
		return &runtime.RemoveContainerResponse{}, nil
	}
	id := container.ID

	// Set removing state to prevent other start/remove operations against this container
	// while it's being removed.
	if err := setContainerRemoving(container); err != nil {
		return nil, errors.Wrapf(err, "failed to set removing state for container %q", id)
	}
	defer func() {
		if retErr != nil {
			// Reset removing if remove failed.
			if err := resetContainerRemoving(container); err != nil {
				logrus.WithError(err).Errorf("failed to reset removing state for container %q", id)
			}
		}
	}()

	// NOTE(random-liu): Docker set container to "Dead" state when start removing the
	// container so as to avoid start/restart the container again. However, for current
	// kubelet implementation, we'll never start a container once we decide to remove it,
	// so we don't need the "Dead" state for now.

	// Delete containerd container.
	if err := container.Container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to delete containerd container %q", id)
		}
		log.Tracef("Remove called for containerd container %q that does not exist", id)
	}

	// Delete container checkpoint.
	if err := container.Delete(); err != nil {
		return nil, errors.Wrapf(err, "failed to delete container checkpoint for %q", id)
	}

	containerRootDir := c.getContainerRootDir(id)
	if err := system.EnsureRemoveAll(containerRootDir); err != nil {
		return nil, errors.Wrapf(err, "failed to remove container root directory %q",
			containerRootDir)
	}
	volatileContainerRootDir := c.getVolatileContainerRootDir(id)
	if err := system.EnsureRemoveAll(volatileContainerRootDir); err != nil {
		return nil, errors.Wrapf(err, "failed to remove volatile container root directory %q",
			volatileContainerRootDir)
	}

	c.containerStore.Delete(id)

	c.containerNameIndex.ReleaseByKey(id)

	return &runtime.RemoveContainerResponse{}, nil
}

// setContainerRemoving sets the container into removing state. In removing state, the
// container will not be started or removed again.
func setContainerRemoving(container containerstore.Container) error {
	return container.Status.Update(func(status containerstore.Status) (containerstore.Status, error) {
		// Do not remove container if it's still running or unknown.
		if status.State() == runtime.ContainerState_CONTAINER_RUNNING {
			return status, errors.New("container is still running, to stop first")
		}
		if status.State() == runtime.ContainerState_CONTAINER_UNKNOWN {
			return status, errors.New("container state is unknown, to stop first")
		}
		if status.Removing {
			return status, errors.New("container is already in removing state")
		}
		status.Removing = true
		return status, nil
	})
}

// resetContainerRemoving resets the container removing state on remove failure. So
// that we could remove the container again.
func resetContainerRemoving(container containerstore.Container) error {
	return container.Status.Update(func(status containerstore.Status) (containerstore.Status, error) {
		status.Removing = false
		return status, nil
	})
}
