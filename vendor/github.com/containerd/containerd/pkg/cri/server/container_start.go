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
	"io"
	"time"

	"github.com/containerd/containerd"
	containerdio "github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/nri"
	v1 "github.com/containerd/nri/types/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	cio "github.com/containerd/containerd/pkg/cri/io"
	containerstore "github.com/containerd/containerd/pkg/cri/store/container"
	sandboxstore "github.com/containerd/containerd/pkg/cri/store/sandbox"
	ctrdutil "github.com/containerd/containerd/pkg/cri/util"
	cioutil "github.com/containerd/containerd/pkg/ioutil"
)

// StartContainer starts the container.
func (c *criService) StartContainer(ctx context.Context, r *runtime.StartContainerRequest) (retRes *runtime.StartContainerResponse, retErr error) {
	cntr, err := c.containerStore.Get(r.GetContainerId())
	if err != nil {
		return nil, errors.Wrapf(err, "an error occurred when try to find container %q", r.GetContainerId())
	}

	id := cntr.ID
	meta := cntr.Metadata
	container := cntr.Container
	config := meta.Config

	// Set starting state to prevent other start/remove operations against this container
	// while it's being started.
	if err := setContainerStarting(cntr); err != nil {
		return nil, errors.Wrapf(err, "failed to set starting state for container %q", id)
	}
	defer func() {
		if retErr != nil {
			// Set container to exited if fail to start.
			if err := cntr.Status.UpdateSync(func(status containerstore.Status) (containerstore.Status, error) {
				status.Pid = 0
				status.FinishedAt = time.Now().UnixNano()
				status.ExitCode = errorStartExitCode
				status.Reason = errorStartReason
				status.Message = retErr.Error()
				return status, nil
			}); err != nil {
				log.G(ctx).WithError(err).Errorf("failed to set start failure state for container %q", id)
			}
		}
		if err := resetContainerStarting(cntr); err != nil {
			log.G(ctx).WithError(err).Errorf("failed to reset starting state for container %q", id)
		}
	}()

	// Get sandbox config from sandbox store.
	sandbox, err := c.sandboxStore.Get(meta.SandboxID)
	if err != nil {
		return nil, errors.Wrapf(err, "sandbox %q not found", meta.SandboxID)
	}
	sandboxID := meta.SandboxID
	if sandbox.Status.Get().State != sandboxstore.StateReady {
		return nil, errors.Errorf("sandbox container %q is not running", sandboxID)
	}

	// Recheck target container validity in Linux namespace options.
	if linux := config.GetLinux(); linux != nil {
		nsOpts := linux.GetSecurityContext().GetNamespaceOptions()
		if nsOpts.GetPid() == runtime.NamespaceMode_TARGET {
			_, err := c.validateTargetContainer(sandboxID, nsOpts.TargetId)
			if err != nil {
				return nil, errors.Wrap(err, "invalid target container")
			}
		}
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
		return nil, errors.Wrap(err, "failed to get container info")
	}

	taskOpts := c.taskOpts(ctrInfo.Runtime.Name)
	task, err := container.NewTask(ctx, ioCreation, taskOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create containerd task")
	}
	defer func() {
		if retErr != nil {
			deferCtx, deferCancel := ctrdutil.DeferContext()
			defer deferCancel()
			// It's possible that task is deleted by event monitor.
			if _, err := task.Delete(deferCtx, WithNRISandboxDelete(sandboxID), containerd.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
				log.G(ctx).WithError(err).Errorf("Failed to delete containerd task %q", id)
			}
		}
	}()

	// wait is a long running background request, no timeout needed.
	exitCh, err := task.Wait(ctrdutil.NamespacedContext())
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for containerd task")
	}
	nric, err := nri.New()
	if err != nil {
		log.G(ctx).WithError(err).Error("unable to create nri client")
	}
	if nric != nil {
		nriSB := &nri.Sandbox{
			ID:     sandboxID,
			Labels: sandbox.Config.Labels,
		}
		if _, err := nric.InvokeWithSandbox(ctx, task, v1.Create, nriSB); err != nil {
			return nil, errors.Wrap(err, "nri invoke")
		}
	}

	// Start containerd task.
	if err := task.Start(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to start containerd task %q", id)
	}

	// Update container start timestamp.
	if err := cntr.Status.UpdateSync(func(status containerstore.Status) (containerstore.Status, error) {
		status.Pid = task.Pid()
		status.StartedAt = time.Now().UnixNano()
		return status, nil
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to update container %q state", id)
	}

	// It handles the TaskExit event and update container state after this.
	c.eventMonitor.startContainerExitMonitor(context.Background(), id, task.Pid(), exitCh)

	return &runtime.StartContainerResponse{}, nil
}

// setContainerStarting sets the container into starting state. In starting state, the
// container will not be removed or started again.
func setContainerStarting(container containerstore.Container) error {
	return container.Status.Update(func(status containerstore.Status) (containerstore.Status, error) {
		// Return error if container is not in created state.
		if status.State() != runtime.ContainerState_CONTAINER_CREATED {
			return status, errors.Errorf("container is in %s state", criContainerStateToString(status.State()))
		}
		// Do not start the container when there is a removal in progress.
		if status.Removing {
			return status, errors.New("container is in removing state, can't be started")
		}
		if status.Starting {
			return status, errors.New("container is already in starting state")
		}
		status.Starting = true
		return status, nil
	})
}

// resetContainerStarting resets the container starting state on start failure. So
// that we could remove the container later.
func resetContainerStarting(container containerstore.Container) error {
	return container.Status.Update(func(status containerstore.Status) (containerstore.Status, error) {
		status.Starting = false
		return status, nil
	})
}

// createContainerLoggers creates container loggers and return write closer for stdout and stderr.
func (c *criService) createContainerLoggers(logPath string, tty bool) (stdout io.WriteCloser, stderr io.WriteCloser, err error) {
	if logPath != "" {
		// Only generate container log when log path is specified.
		f, err := openLogFile(logPath)
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
