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
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/docker/docker/pkg/signal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	containerstore "github.com/containerd/cri/pkg/store/container"
)

// killContainerTimeout is the timeout that we wait for the container to
// be SIGKILLed.
// The timeout is set to 1 min, because the default CRI operation timeout
// for StopContainer is (2 min + stop timeout). Set to 1 min, so that we
// have enough time for kill(all=true) and kill(all=false).
const killContainerTimeout = 1 * time.Minute

// StopContainer stops a running container with a grace period (i.e., timeout).
func (c *criService) StopContainer(ctx context.Context, r *runtime.StopContainerRequest) (*runtime.StopContainerResponse, error) {
	// Get container config from container store.
	container, err := c.containerStore.Get(r.GetContainerId())
	if err != nil {
		return nil, errors.Wrapf(err, "an error occurred when try to find container %q", r.GetContainerId())
	}

	if err := c.stopContainer(ctx, container, time.Duration(r.GetTimeout())*time.Second); err != nil {
		return nil, err
	}

	return &runtime.StopContainerResponse{}, nil
}

// stopContainer stops a container based on the container metadata.
func (c *criService) stopContainer(ctx context.Context, container containerstore.Container, timeout time.Duration) error {
	id := container.ID

	// Return without error if container is not running. This makes sure that
	// stop only takes real action after the container is started.
	state := container.Status.Get().State()
	if state != runtime.ContainerState_CONTAINER_RUNNING {
		logrus.Infof("Container to stop %q is not running, current state %q",
			id, criContainerStateToString(state))
		return nil
	}

	task, err := container.Container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to stop container, task not found for container %q", id)
		}
		return nil
	}

	// We only need to kill the task. The event handler will Delete the
	// task from containerd after it handles the Exited event.
	if timeout > 0 {
		stopSignal := unix.SIGTERM
		image, err := c.imageStore.Get(container.ImageRef)
		if err != nil {
			// NOTE(random-liu): It's possible that the container is stopped,
			// deleted and image is garbage collected before this point. However,
			// the chance is really slim, even it happens, it's still fine to return
			// an error here.
			return errors.Wrapf(err, "failed to get image metadata %q", container.ImageRef)
		}
		if image.ImageSpec.Config.StopSignal != "" {
			stopSignal, err = signal.ParseSignal(image.ImageSpec.Config.StopSignal)
			if err != nil {
				return errors.Wrapf(err, "failed to parse stop signal %q",
					image.ImageSpec.Config.StopSignal)
			}
		}
		logrus.Infof("Stop container %q with signal %v", id, stopSignal)
		if err = task.Kill(ctx, stopSignal); err != nil && !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to stop container %q", id)
		}

		if err = c.waitContainerStop(ctx, container, timeout); err == nil {
			return nil
		}
		logrus.WithError(err).Errorf("An error occurs during waiting for container %q to be stopped", id)
	}

	logrus.Infof("Kill container %q", id)
	if err = task.Kill(ctx, unix.SIGKILL); err != nil && !errdefs.IsNotFound(err) {
		return errors.Wrapf(err, "failed to kill container %q", id)
	}

	// Wait for a fixed timeout until container stop is observed by event monitor.
	if err = c.waitContainerStop(ctx, container, killContainerTimeout); err == nil {
		return nil
	}
	return errors.Wrapf(err, "an error occurs during waiting for container %q to be killed", id)
}

// waitContainerStop waits for container to be stopped until timeout exceeds or context is cancelled.
func (c *criService) waitContainerStop(ctx context.Context, container containerstore.Container, timeout time.Duration) error {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	select {
	case <-ctx.Done():
		return errors.Errorf("wait container %q is cancelled", container.ID)
	case <-timeoutTimer.C:
		return errors.Errorf("wait container %q stop timeout", container.ID)
	case <-container.Stopped():
		return nil
	}
}
