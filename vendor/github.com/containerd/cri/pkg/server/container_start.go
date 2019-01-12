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
	"io"
	"os"
	"time"

	"github.com/containerd/containerd"
	containerdio "github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	cioutil "github.com/containerd/cri/pkg/ioutil"
	cio "github.com/containerd/cri/pkg/server/io"
	containerstore "github.com/containerd/cri/pkg/store/container"
	sandboxstore "github.com/containerd/cri/pkg/store/sandbox"
)

// StartContainer starts the container.
func (c *criService) StartContainer(ctx context.Context, r *runtime.StartContainerRequest) (retRes *runtime.StartContainerResponse, retErr error) {
	container, err := c.containerStore.Get(r.GetContainerId())
	if err != nil {
		return nil, errors.Wrapf(err, "an error occurred when try to find container %q", r.GetContainerId())
	}

	var startErr error
	// update container status in one transaction to avoid race with event monitor.
	if err := container.Status.UpdateSync(func(status containerstore.Status) (containerstore.Status, error) {
		// Always apply status change no matter startContainer fails or not. Because startContainer
		// may change container state no matter it fails or succeeds.
		startErr = c.startContainer(ctx, container, &status)
		return status, nil
	}); startErr != nil {
		return nil, startErr
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to update container %q metadata", container.ID)
	}
	return &runtime.StartContainerResponse{}, nil
}

// startContainer actually starts the container. The function needs to be run in one transaction. Any updates
// to the status passed in will be applied no matter the function returns error or not.
func (c *criService) startContainer(ctx context.Context,
	cntr containerstore.Container,
	status *containerstore.Status) (retErr error) {
	id := cntr.ID
	meta := cntr.Metadata
	container := cntr.Container
	config := meta.Config

	// Return error if container is not in created state.
	if status.State() != runtime.ContainerState_CONTAINER_CREATED {
		return errors.Errorf("container %q is in %s state", id, criContainerStateToString(status.State()))
	}
	// Do not start the container when there is a removal in progress.
	if status.Removing {
		return errors.Errorf("container %q is in removing state", id)
	}

	defer func() {
		if retErr != nil {
			// Set container to exited if fail to start.
			status.Pid = 0
			status.FinishedAt = time.Now().UnixNano()
			status.ExitCode = errorStartExitCode
			status.Reason = errorStartReason
			status.Message = retErr.Error()
		}
	}()

	// Get sandbox config from sandbox store.
	sandbox, err := c.sandboxStore.Get(meta.SandboxID)
	if err != nil {
		return errors.Wrapf(err, "sandbox %q not found", meta.SandboxID)
	}
	sandboxID := meta.SandboxID
	if sandbox.Status.Get().State != sandboxstore.StateReady {
		return errors.Errorf("sandbox container %q is not running", sandboxID)
	}

	ioCreation := func(id string) (_ containerdio.IO, err error) {
		stdoutWC, stderrWC, err := c.createContainerLoggers(meta.LogPath, config.GetTty())
		if err != nil {
			return nil, errors.Wrap(err, "failed to create container loggers")
		}
		cntr.IO.AddOutput("log", stdoutWC, stderrWC)
		cntr.IO.Pipe()
		return cntr.IO, nil
	}

	ctrInfo, err := container.Info(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get container info")
	}

	var taskOpts []containerd.NewTaskOpts
	// TODO(random-liu): Remove this after shim v1 is deprecated.
	if c.config.NoPivot && ctrInfo.Runtime.Name == linuxRuntime {
		taskOpts = append(taskOpts, containerd.WithNoPivotRoot)
	}
	task, err := container.NewTask(ctx, ioCreation, taskOpts...)
	if err != nil {
		return errors.Wrap(err, "failed to create containerd task")
	}
	defer func() {
		if retErr != nil {
			deferCtx, deferCancel := ctrdutil.DeferContext()
			defer deferCancel()
			// It's possible that task is deleted by event monitor.
			if _, err := task.Delete(deferCtx, containerd.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
				logrus.WithError(err).Errorf("Failed to delete containerd task %q", id)
			}
		}
	}()

	// Start containerd task.
	if err := task.Start(ctx); err != nil {
		return errors.Wrapf(err, "failed to start containerd task %q", id)
	}

	// Update container start timestamp.
	status.Pid = task.Pid()
	status.StartedAt = time.Now().UnixNano()
	return nil
}

// createContainerLoggers creates container loggers and return write closer for stdout and stderr.
func (c *criService) createContainerLoggers(logPath string, tty bool) (stdout io.WriteCloser, stderr io.WriteCloser, err error) {
	if logPath != "" {
		// Only generate container log when log path is specified.
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to create and open log file")
		}
		defer func() {
			if err != nil {
				f.Close()
			}
		}()
		var stdoutCh, stderrCh <-chan struct{}
		wc := cioutil.NewSerialWriteCloser(f)
		stdout, stdoutCh = cio.NewCRILogger(logPath, wc, cio.Stdout, c.config.MaxContainerLogLineSize)
		// Only redirect stderr when there is no tty.
		if !tty {
			stderr, stderrCh = cio.NewCRILogger(logPath, wc, cio.Stderr, c.config.MaxContainerLogLineSize)
		}
		go func() {
			if stdoutCh != nil {
				<-stdoutCh
			}
			if stderrCh != nil {
				<-stderrCh
			}
			logrus.Debugf("Finish redirecting log file %q, closing it", logPath)
			f.Close()
		}()
	} else {
		stdout = cio.NewDiscardLogger()
		stderr = cio.NewDiscardLogger()
	}
	return
}
