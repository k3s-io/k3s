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

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	sandboxstore "github.com/containerd/cri/pkg/store/sandbox"
)

// PodSandboxStatus returns the status of the PodSandbox.
func (c *criService) PodSandboxStatus(ctx context.Context, r *runtime.PodSandboxStatusRequest) (*runtime.PodSandboxStatusResponse, error) {
	sandbox, err := c.sandboxStore.Get(r.GetPodSandboxId())
	if err != nil {
		return nil, errors.Wrap(err, "an error occurred when try to find sandbox")
	}

	ip, err := c.getIP(sandbox)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sandbox ip")
	}
	status := toCRISandboxStatus(sandbox.Metadata, sandbox.Status.Get(), ip)
	if status.GetCreatedAt() == 0 {
		// CRI doesn't allow CreatedAt == 0.
		info, err := sandbox.Container.Info(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get CreatedAt for sandbox container in %q state", status.State)
		}
		status.CreatedAt = info.CreatedAt.UnixNano()
	}
	if !r.GetVerbose() {
		return &runtime.PodSandboxStatusResponse{Status: status}, nil
	}

	// Generate verbose information.
	info, err := toCRISandboxInfo(ctx, sandbox)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get verbose sandbox container info")
	}

	return &runtime.PodSandboxStatusResponse{
		Status: status,
		Info:   info,
	}, nil
}

func (c *criService) getIP(sandbox sandboxstore.Sandbox) (string, error) {
	config := sandbox.Config

	if config.GetLinux().GetSecurityContext().GetNamespaceOptions().GetNetwork() == runtime.NamespaceMode_NODE {
		// For sandboxes using the node network we are not
		// responsible for reporting the IP.
		return "", nil
	}

	if closed, err := sandbox.NetNS.Closed(); err != nil {
		return "", errors.Wrap(err, "check network namespace closed")
	} else if closed {
		return "", nil
	}

	return sandbox.IP, nil
}

// toCRISandboxStatus converts sandbox metadata into CRI pod sandbox status.
func toCRISandboxStatus(meta sandboxstore.Metadata, status sandboxstore.Status, ip string) *runtime.PodSandboxStatus {
	// Set sandbox state to NOTREADY by default.
	state := runtime.PodSandboxState_SANDBOX_NOTREADY
	if status.State == sandboxstore.StateReady {
		state = runtime.PodSandboxState_SANDBOX_READY
	}
	nsOpts := meta.Config.GetLinux().GetSecurityContext().GetNamespaceOptions()
	return &runtime.PodSandboxStatus{
		Id:        meta.ID,
		Metadata:  meta.Config.GetMetadata(),
		State:     state,
		CreatedAt: status.CreatedAt.UnixNano(),
		Network:   &runtime.PodSandboxNetworkStatus{Ip: ip},
		Linux: &runtime.LinuxPodSandboxStatus{
			Namespaces: &runtime.Namespace{
				Options: &runtime.NamespaceOption{
					Network: nsOpts.GetNetwork(),
					Pid:     nsOpts.GetPid(),
					Ipc:     nsOpts.GetIpc(),
				},
			},
		},
		Labels:      meta.Config.GetLabels(),
		Annotations: meta.Config.GetAnnotations(),
	}
}

// SandboxInfo is extra information for sandbox.
// TODO (mikebrow): discuss predefining constants structures for some or all of these field names in CRI
type SandboxInfo struct {
	Pid            uint32                    `json:"pid"`
	Status         string                    `json:"processStatus"`
	NetNSClosed    bool                      `json:"netNamespaceClosed"`
	Image          string                    `json:"image"`
	SnapshotKey    string                    `json:"snapshotKey"`
	Snapshotter    string                    `json:"snapshotter"`
	RuntimeHandler string                    `json:"runtimeHandler"`
	RuntimeType    string                    `json:"runtimeType"`
	RuntimeOptions interface{}               `json:"runtimeOptions"`
	Config         *runtime.PodSandboxConfig `json:"config"`
	RuntimeSpec    *runtimespec.Spec         `json:"runtimeSpec"`
}

// toCRISandboxInfo converts internal container object information to CRI sandbox status response info map.
func toCRISandboxInfo(ctx context.Context, sandbox sandboxstore.Sandbox) (map[string]string, error) {
	container := sandbox.Container
	task, err := container.Task(ctx, nil)
	if err != nil && !errdefs.IsNotFound(err) {
		return nil, errors.Wrap(err, "failed to get sandbox container task")
	}

	var processStatus containerd.ProcessStatus
	if task != nil {
		taskStatus, err := task.Status(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get task status")
		}

		processStatus = taskStatus.Status
	}

	si := &SandboxInfo{
		Pid:            sandbox.Status.Get().Pid,
		RuntimeHandler: sandbox.RuntimeHandler,
		Status:         string(processStatus),
		Config:         sandbox.Config,
	}

	if si.Status == "" {
		// If processStatus is empty, it means that the task is deleted. Apply "deleted"
		// status which does not exist in containerd.
		si.Status = "deleted"
	}

	if sandbox.NetNS != nil {
		// Add network closed information if sandbox is not using host network.
		closed, err := sandbox.NetNS.Closed()
		if err != nil {
			return nil, errors.Wrap(err, "failed to check network namespace closed")
		}
		si.NetNSClosed = closed
	}

	spec, err := container.Spec(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sandbox container runtime spec")
	}
	si.RuntimeSpec = spec

	ctrInfo, err := container.Info(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get sandbox container info")
	}
	// Do not use config.SandboxImage because the configuration might
	// be changed during restart. It may not reflect the actual image
	// used by the sandbox container.
	si.Image = ctrInfo.Image
	si.SnapshotKey = ctrInfo.SnapshotKey
	si.Snapshotter = ctrInfo.Snapshotter

	runtimeOptions, err := getRuntimeOptions(ctrInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get runtime options")
	}
	si.RuntimeType = ctrInfo.Runtime.Name
	si.RuntimeOptions = runtimeOptions

	infoBytes, err := json.Marshal(si)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to marshal info %v", si)
	}
	return map[string]string{
		"info": string(infoBytes),
	}, nil
}
