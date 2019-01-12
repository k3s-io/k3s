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
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/log"
	"github.com/containerd/cri/pkg/store"
	sandboxstore "github.com/containerd/cri/pkg/store/sandbox"
)

// RemovePodSandbox removes the sandbox. If there are running containers in the
// sandbox, they should be forcibly removed.
func (c *criService) RemovePodSandbox(ctx context.Context, r *runtime.RemovePodSandboxRequest) (*runtime.RemovePodSandboxResponse, error) {
	sandbox, err := c.sandboxStore.Get(r.GetPodSandboxId())
	if err != nil {
		if err != store.ErrNotExist {
			return nil, errors.Wrapf(err, "an error occurred when try to find sandbox %q",
				r.GetPodSandboxId())
		}
		// Do not return error if the id doesn't exist.
		log.Tracef("RemovePodSandbox called for sandbox %q that does not exist",
			r.GetPodSandboxId())
		return &runtime.RemovePodSandboxResponse{}, nil
	}
	// Use the full sandbox id.
	id := sandbox.ID

	// Return error if sandbox container is still running.
	if sandbox.Status.Get().State == sandboxstore.StateReady {
		return nil, errors.Errorf("sandbox container %q is not fully stopped", id)
	}

	// Return error if sandbox network namespace is not closed yet.
	if sandbox.NetNS != nil {
		nsPath := sandbox.NetNS.GetPath()
		if closed, err := sandbox.NetNS.Closed(); err != nil {
			return nil, errors.Wrapf(err, "failed to check sandbox network namespace %q closed", nsPath)
		} else if !closed {
			return nil, errors.Errorf("sandbox network namespace %q is not fully closed", nsPath)
		}
	}

	// Remove all containers inside the sandbox.
	// NOTE(random-liu): container could still be created after this point, Kubelet should
	// not rely on this behavior.
	// TODO(random-liu): Introduce an intermediate state to avoid container creation after
	// this point.
	cntrs := c.containerStore.List()
	for _, cntr := range cntrs {
		if cntr.SandboxID != id {
			continue
		}
		_, err = c.RemoveContainer(ctx, &runtime.RemoveContainerRequest{ContainerId: cntr.ID})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to remove container %q", cntr.ID)
		}
	}

	// Cleanup the sandbox root directories.
	sandboxRootDir := c.getSandboxRootDir(id)
	if err := system.EnsureRemoveAll(sandboxRootDir); err != nil {
		return nil, errors.Wrapf(err, "failed to remove sandbox root directory %q",
			sandboxRootDir)
	}
	volatileSandboxRootDir := c.getVolatileSandboxRootDir(id)
	if err := system.EnsureRemoveAll(volatileSandboxRootDir); err != nil {
		return nil, errors.Wrapf(err, "failed to remove volatile sandbox root directory %q",
			volatileSandboxRootDir)
	}

	// Delete sandbox container.
	if err := sandbox.Container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to delete sandbox container %q", id)
		}
		log.Tracef("Remove called for sandbox container %q that does not exist", id)
	}

	// Remove sandbox from sandbox store. Note that once the sandbox is successfully
	// deleted:
	// 1) ListPodSandbox will not include this sandbox.
	// 2) PodSandboxStatus and StopPodSandbox will return error.
	// 3) On-going operations which have held the reference will not be affected.
	c.sandboxStore.Delete(id)

	// Release the sandbox name reserved for the sandbox.
	c.sandboxNameIndex.ReleaseByKey(id)

	return &runtime.RemovePodSandboxResponse{}, nil
}
