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

	eventtypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	sandboxstore "github.com/containerd/cri/pkg/store/sandbox"
)

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be forcibly terminated.
func (c *criService) StopPodSandbox(ctx context.Context, r *runtime.StopPodSandboxRequest) (*runtime.StopPodSandboxResponse, error) {
	sandbox, err := c.sandboxStore.Get(r.GetPodSandboxId())
	if err != nil {
		return nil, errors.Wrapf(err, "an error occurred when try to find sandbox %q",
			r.GetPodSandboxId())
	}

	if err := c.stopPodSandbox(ctx, sandbox); err != nil {
		return nil, err
	}

	return &runtime.StopPodSandboxResponse{}, nil
}

func (c *criService) stopPodSandbox(ctx context.Context, sandbox sandboxstore.Sandbox) error {
	// Use the full sandbox id.
	id := sandbox.ID

	// Stop all containers inside the sandbox. This terminates the container forcibly,
	// and container may still be created, so production should not rely on this behavior.
	// TODO(random-liu): Introduce a state in sandbox to avoid future container creation.
	containers := c.containerStore.List()
	for _, container := range containers {
		if container.SandboxID != id {
			continue
		}
		// Forcibly stop the container. Do not use `StopContainer`, because it introduces a race
		// if a container is removed after list.
		if err := c.stopContainer(ctx, container, 0); err != nil {
			return errors.Wrapf(err, "failed to stop container %q", container.ID)
		}
	}

	if err := c.cleanupSandboxFiles(id, sandbox.Config); err != nil {
		return errors.Wrap(err, "failed to cleanup sandbox files")
	}

	// Only stop sandbox container when it's running or unknown.
	state := sandbox.Status.Get().State
	if state == sandboxstore.StateReady || state == sandboxstore.StateUnknown {
		if err := c.stopSandboxContainer(ctx, sandbox); err != nil {
			return errors.Wrapf(err, "failed to stop sandbox container %q in %q state", id, state)
		}
	}

	// Teardown network for sandbox.
	if sandbox.NetNS != nil {
		// Use empty netns path if netns is not available. This is defined in:
		// https://github.com/containernetworking/cni/blob/v0.7.0-alpha1/SPEC.md
		if closed, err := sandbox.NetNS.Closed(); err != nil {
			return errors.Wrap(err, "failed to check network namespace closed")
		} else if closed {
			sandbox.NetNSPath = ""
		}
		if err := c.teardownPodNetwork(ctx, sandbox); err != nil {
			return errors.Wrapf(err, "failed to destroy network for sandbox %q", id)
		}
		if err := sandbox.NetNS.Remove(); err != nil {
			return errors.Wrapf(err, "failed to remove network namespace for sandbox %q", id)
		}
	}

	log.G(ctx).Infof("TearDown network for sandbox %q successfully", id)

	return nil
}

// stopSandboxContainer kills the sandbox container.
// `task.Delete` is not called here because it will be called when
// the event monitor handles the `TaskExit` event.
func (c *criService) stopSandboxContainer(ctx context.Context, sandbox sandboxstore.Sandbox) error {
	id := sandbox.ID
	container := sandbox.Container
	state := sandbox.Status.Get().State
	task, err := container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return errors.Wrap(err, "failed to get sandbox container")
		}
		// Don't return for unknown state, some cleanup needs to be done.
		if state == sandboxstore.StateUnknown {
			return cleanupUnknownSandbox(ctx, id, sandbox)
		}
		return nil
	}

	// Handle unknown state.
	// The cleanup logic is the same with container unknown state.
	if state == sandboxstore.StateUnknown {
		// Start an exit handler for containers in unknown state.
		waitCtx, waitCancel := context.WithCancel(ctrdutil.NamespacedContext())
		defer waitCancel()
		exitCh, err := task.Wait(waitCtx)
		if err != nil {
			if !errdefs.IsNotFound(err) {
				return errors.Wrap(err, "failed to wait for task")
			}
			return cleanupUnknownSandbox(ctx, id, sandbox)
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

	// Kill the sandbox container.
	if err = task.Kill(ctx, syscall.SIGKILL); err != nil && !errdefs.IsNotFound(err) {
		return errors.Wrap(err, "failed to kill sandbox container")
	}

	return c.waitSandboxStop(ctx, sandbox)
}

// waitSandboxStop waits for sandbox to be stopped until context is cancelled or
// the context deadline is exceeded.
func (c *criService) waitSandboxStop(ctx context.Context, sandbox sandboxstore.Sandbox) error {
	select {
	case <-ctx.Done():
		return errors.Wrapf(ctx.Err(), "wait sandbox container %q", sandbox.ID)
	case <-sandbox.Stopped():
		return nil
	}
}

// teardownPodNetwork removes the network from the pod
func (c *criService) teardownPodNetwork(ctx context.Context, sandbox sandboxstore.Sandbox) error {
	if c.netPlugin == nil {
		return errors.New("cni config not initialized")
	}

	var (
		id     = sandbox.ID
		path   = sandbox.NetNSPath
		config = sandbox.Config
	)
	opts, err := cniNamespaceOpts(id, config)
	if err != nil {
		return errors.Wrap(err, "get cni namespace options")
	}

	return c.netPlugin.Remove(ctx, id, path, opts...)
}

// cleanupUnknownSandbox cleanup stopped sandbox in unknown state.
func cleanupUnknownSandbox(ctx context.Context, id string, sandbox sandboxstore.Sandbox) error {
	// Reuse handleSandboxExit to do the cleanup.
	return handleSandboxExit(ctx, &eventtypes.TaskExit{
		ContainerID: id,
		ID:          id,
		Pid:         0,
		ExitStatus:  unknownExitCode,
		ExitedAt:    time.Now(),
	}, sandbox)
}
