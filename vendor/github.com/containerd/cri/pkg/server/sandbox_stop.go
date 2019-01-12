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
	cni "github.com/containerd/go-cni"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

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
	// Use the full sandbox id.
	id := sandbox.ID

	// Stop all containers inside the sandbox. This terminates the container forcibly,
	// and container may still be so production should not rely on this behavior.
	// TODO(random-liu): Delete the sandbox container before this after permanent network namespace
	// is introduced, so that no container will be started after that.
	containers := c.containerStore.List()
	for _, container := range containers {
		if container.SandboxID != id {
			continue
		}
		// Forcibly stop the container. Do not use `StopContainer`, because it introduces a race
		// if a container is removed after list.
		if err = c.stopContainer(ctx, container, 0); err != nil {
			return nil, errors.Wrapf(err, "failed to stop container %q", container.ID)
		}
	}

	// Teardown network for sandbox.
	if sandbox.NetNS != nil {
		netNSPath := sandbox.NetNSPath
		// Use empty netns path if netns is not available. This is defined in:
		// https://github.com/containernetworking/cni/blob/v0.7.0-alpha1/SPEC.md
		if closed, err := sandbox.NetNS.Closed(); err != nil {
			return nil, errors.Wrap(err, "failed to check network namespace closed")
		} else if closed {
			netNSPath = ""
		}
		if err := c.teardownPod(id, netNSPath, sandbox.Config); err != nil {
			return nil, errors.Wrapf(err, "failed to destroy network for sandbox %q", id)
		}
		if err = sandbox.NetNS.Remove(); err != nil {
			return nil, errors.Wrapf(err, "failed to remove network namespace for sandbox %q", id)
		}
	}

	logrus.Infof("TearDown network for sandbox %q successfully", id)

	if err := c.unmountSandboxFiles(id, sandbox.Config); err != nil {
		return nil, errors.Wrap(err, "failed to unmount sandbox files")
	}

	// Only stop sandbox container when it's running.
	if sandbox.Status.Get().State == sandboxstore.StateReady {
		if err := c.stopSandboxContainer(ctx, sandbox); err != nil {
			return nil, errors.Wrapf(err, "failed to stop sandbox container %q", id)
		}
	}
	return &runtime.StopPodSandboxResponse{}, nil
}

// stopSandboxContainer kills the sandbox container.
// `task.Delete` is not called here because it will be called when
// the event monitor handles the `TaskExit` event.
func (c *criService) stopSandboxContainer(ctx context.Context, sandbox sandboxstore.Sandbox) error {
	container := sandbox.Container
	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "failed to get sandbox container")
	}

	// Kill the sandbox container.
	if err = task.Kill(ctx, unix.SIGKILL); err != nil && !errdefs.IsNotFound(err) {
		return errors.Wrap(err, "failed to kill sandbox container")
	}

	return c.waitSandboxStop(ctx, sandbox, killContainerTimeout)
}

// waitSandboxStop waits for sandbox to be stopped until timeout exceeds or context is cancelled.
func (c *criService) waitSandboxStop(ctx context.Context, sandbox sandboxstore.Sandbox, timeout time.Duration) error {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	select {
	case <-ctx.Done():
		return errors.Errorf("wait sandbox container %q is cancelled", sandbox.ID)
	case <-timeoutTimer.C:
		return errors.Errorf("wait sandbox container %q stop timeout", sandbox.ID)
	case <-sandbox.Stopped():
		return nil
	}
}

// teardownPod removes the network from the pod
func (c *criService) teardownPod(id string, path string, config *runtime.PodSandboxConfig) error {
	if c.netPlugin == nil {
		return errors.New("cni config not intialized")
	}

	labels := getPodCNILabels(id, config)
	return c.netPlugin.Remove(id,
		path,
		cni.WithLabels(labels),
		cni.WithCapabilityPortMap(toCNIPortMappings(config.GetPortMappings())))
}
