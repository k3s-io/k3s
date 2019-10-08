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
	"bytes"
	"io"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	containerdio "github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/oci"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"k8s.io/client-go/tools/remotecommand"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	cioutil "github.com/containerd/cri/pkg/ioutil"
	cio "github.com/containerd/cri/pkg/server/io"
	"github.com/containerd/cri/pkg/util"
)

// ExecSync executes a command in the container, and returns the stdout output.
// If command exits with a non-zero exit code, an error is returned.
func (c *criService) ExecSync(ctx context.Context, r *runtime.ExecSyncRequest) (*runtime.ExecSyncResponse, error) {
	var stdout, stderr bytes.Buffer
	exitCode, err := c.execInContainer(ctx, r.GetContainerId(), execOptions{
		cmd:     r.GetCmd(),
		stdout:  cioutil.NewNopWriteCloser(&stdout),
		stderr:  cioutil.NewNopWriteCloser(&stderr),
		timeout: time.Duration(r.GetTimeout()) * time.Second,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to exec in container")
	}

	return &runtime.ExecSyncResponse{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: int32(*exitCode),
	}, nil
}

// execOptions specifies how to execute command in container.
type execOptions struct {
	cmd     []string
	stdin   io.Reader
	stdout  io.WriteCloser
	stderr  io.WriteCloser
	tty     bool
	resize  <-chan remotecommand.TerminalSize
	timeout time.Duration
}

// execInContainer executes a command inside the container synchronously, and
// redirects stdio stream properly.
func (c *criService) execInContainer(ctx context.Context, id string, opts execOptions) (*uint32, error) {
	// Cancel the context before returning to ensure goroutines are stopped.
	// This is important, because if `Start` returns error, `Wait` will hang
	// forever unless we cancel the context.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Get container from our container store.
	cntr, err := c.containerStore.Get(id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find container %q in store", id)
	}
	id = cntr.ID

	state := cntr.Status.Get().State()
	if state != runtime.ContainerState_CONTAINER_RUNNING {
		return nil, errors.Errorf("container is in %s state", criContainerStateToString(state))
	}

	container := cntr.Container
	spec, err := container.Spec(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get container spec")
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load task")
	}
	pspec := spec.Process

	pspec.Terminal = opts.tty
	if opts.tty {
		if err := oci.WithEnv([]string{"TERM=xterm"})(ctx, nil, nil, spec); err != nil {
			return nil, errors.Wrap(err, "add TERM env var to spec")
		}
	}

	pspec.Args = opts.cmd

	if opts.stdout == nil {
		opts.stdout = cio.NewDiscardLogger()
	}
	if opts.stderr == nil {
		opts.stderr = cio.NewDiscardLogger()
	}
	execID := util.GenerateID()
	log.G(ctx).Debugf("Generated exec id %q for container %q", execID, id)
	volatileRootDir := c.getVolatileContainerRootDir(id)
	var execIO *cio.ExecIO
	process, err := task.Exec(ctx, execID, pspec,
		func(id string) (containerdio.IO, error) {
			var err error
			execIO, err = cio.NewExecIO(id, volatileRootDir, opts.tty, opts.stdin != nil)
			return execIO, err
		},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create exec %q", execID)
	}
	defer func() {
		deferCtx, deferCancel := ctrdutil.DeferContext()
		defer deferCancel()
		if _, err := process.Delete(deferCtx, containerd.WithProcessKill); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to delete exec process %q for container %q", execID, id)
		}
	}()

	exitCh, err := process.Wait(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to wait for process %q", execID)
	}
	if err := process.Start(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to start exec %q", execID)
	}

	handleResizing(opts.resize, func(size remotecommand.TerminalSize) {
		if err := process.Resize(ctx, uint32(size.Width), uint32(size.Height)); err != nil {
			log.G(ctx).WithError(err).Errorf("Failed to resize process %q console for container %q", execID, id)
		}
	})

	attachDone := execIO.Attach(cio.AttachOptions{
		Stdin:     opts.stdin,
		Stdout:    opts.stdout,
		Stderr:    opts.stderr,
		Tty:       opts.tty,
		StdinOnce: true,
		CloseStdin: func() error {
			return process.CloseIO(ctx, containerd.WithStdinCloser)
		},
	})

	execCtx := ctx
	if opts.timeout > 0 {
		var execCtxCancel context.CancelFunc
		execCtx, execCtxCancel = context.WithTimeout(ctx, opts.timeout)
		defer execCtxCancel()
	}

	select {
	case <-execCtx.Done():
		// Ignore the not found error because the process may exit itself before killing.
		if err := process.Kill(ctx, syscall.SIGKILL); err != nil && !errdefs.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to kill exec %q", execID)
		}
		// Wait for the process to be killed.
		exitRes := <-exitCh
		log.G(ctx).Infof("Timeout received while waiting for exec process kill %q code %d and error %v",
			execID, exitRes.ExitCode(), exitRes.Error())
		<-attachDone
		log.G(ctx).Debugf("Stream pipe for exec process %q done", execID)
		return nil, errors.Wrapf(execCtx.Err(), "timeout %v exceeded", opts.timeout)
	case exitRes := <-exitCh:
		code, _, err := exitRes.Result()
		log.G(ctx).Infof("Exec process %q exits with exit code %d and error %v", execID, code, err)
		if err != nil {
			return nil, errors.Wrapf(err, "failed while waiting for exec %q", execID)
		}
		<-attachDone
		log.G(ctx).Debugf("Stream pipe for exec process %q done", execID)
		return &code, nil
	}
}
