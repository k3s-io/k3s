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
	"syscall"
	"time"

	"github.com/containerd/containerd"
	eventtypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	"github.com/containerd/cri/pkg/store"
	containerstore "github.com/containerd/cri/pkg/store/container"
)

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
	if state != runtime.ContainerState_CONTAINER_RUNNING &&
		state != runtime.ContainerState_CONTAINER_UNKNOWN {
		log.G(ctx).Infof("Container to stop %q must be in running or unknown state, current state %q",
			id, criContainerStateToString(state))
		return nil
	}

	task, err := container.Container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get task for container %q", id)
		}
		// Don't return for unknown state, some cleanup needs to be done.
		if state == runtime.ContainerState_CONTAINER_UNKNOWN {
			return cleanupUnknownContainer(ctx, id, container)
		}
		return nil
	}

	// Handle unknown state.
	if state == runtime.ContainerState_CONTAINER_UNKNOWN {
		// Start an exit handler for containers in unknown state.
		waitCtx, waitCancel := context.WithCancel(ctrdutil.NamespacedContext())
		defer waitCancel()
		exitCh, err := task.Wait(waitCtx)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return errors.Wrapf(err, "failed to wait for task for %q", id)
			}
			return cleanupUnknownContainer(ctx, id, container)
		}

		exitCtx, exitCancel := context.WithCancel(context.Background())
		stopCh := c.eventMonitor.startExitMonitor(exitCtx, id, task.Pid(), exitCh)
		defer func() {
			exitCancel()
			// This ensures that exit monitor is stopped before
			// `Wait` is cancelled, so no exit event is generated
			// because of the `Wait` cancellation.
			<-stopCh
		}()
	}

	// We only need to kill the task. The event handler will Delete the
	// task from containerd after it handles the Exited event.
	if timeout > 0 {
		stopSignal := "SIGTERM"
		if container.StopSignal != "" {
			stopSignal = container.StopSignal
		} else {
			// The image may have been deleted, and the `StopSignal` field is
			// just introduced to handle that.
			// However, for containers created before the `StopSignal` field is
			// introduced, still try to get the stop signal from the image config.
			// If the image has been deleted, logging an error and using the
			// default SIGTERM is still better than returning error and leaving
			// the container unstoppable. (See issue #990)
			// TODO(random-liu): Remove this logic when containerd 1.2 is deprecated.
			image, err := c.imageStore.Get(container.ImageRef)
			if err != nil {
				if err != store.ErrNotExist {
					return errors.Wrapf(err, "failed to get image %q", container.ImageRef)
				}
				log.G(ctx).Warningf("Image %q not found, stop container with signal %q", container.ImageRef, stopSignal)
			} else {
				if image.ImageSpec.Config.StopSignal != "" {
					stopSignal = image.ImageSpec.Config.StopSignal
				}
			}
		}
		sig, err := containerd.ParseSignal(stopSignal)
		if err != nil {
			return errors.Wrapf(err, "failed to parse stop signal %q", stopSignal)
		}
		log.G(ctx).Infof("Stop container %q with signal %v", id, sig)
		if err = task.Kill(ctx, sig); err != nil && !errdefs.IsNotFound(err) {
			return errors.Wrapf(err, "failed to stop container %q", id)
		}

		sigTermCtx, sigTermCtxCancel := context.WithTimeout(ctx, timeout)
		defer sigTermCtxCancel()
		err = c.waitContainerStop(sigTermCtx, container)
		if err == nil {
			// Container stopped on first signal no need for SIGKILL
			return nil
		}
		// If the parent context was cancelled or exceeded return immediately
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// sigTermCtx was exceeded. Send SIGKILL
		log.G(ctx).Debugf("Stop container %q with signal %v timed out", id, sig)
	}

	log.G(ctx).Infof("Kill container %q", id)
	if err = task.Kill(ctx, syscall.SIGKILL); err != nil && !errdefs.IsNotFound(err) {
		return errors.Wrapf(err, "failed to kill container %q", id)
	}

	// Wait for a fixed timeout until container stop is observed by event monitor.
	err = c.waitContainerStop(ctx, container)
	if err != nil {
		return errors.Wrapf(err, "an error occurs during waiting for container %q to be killed", id)
	}
	return nil
}

// waitContainerStop waits for container to be stopped until context is
// cancelled or the context deadline is exceeded.
func (c *criService) waitContainerStop(ctx context.Context, container containerstore.Container) error {
	select {
	case <-ctx.Done():
		return errors.Wrapf(ctx.Err(), "wait container %q", container.ID)
	case <-container.Stopped():
		return nil
	}
}

// cleanupUnknownContainer cleanup stopped container in unknown state.
func cleanupUnknownContainer(ctx context.Context, id string, cntr containerstore.Container) error {
	// Reuse handleContainerExit to do the cleanup.
	return handleContainerExit(ctx, &eventtypes.TaskExit{
		ContainerID: id,
		ID:          id,
		Pid:         0,
		ExitStatus:  unknownExitCode,
		ExitedAt:    time.Now(),
	}, cntr)
}
