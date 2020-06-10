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

package v2

import (
	"context"

	tasktypes "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/ttrpc"
	"github.com/pkg/errors"
)

type process struct {
	id   string
	shim *shim
}

func (p *process) ID() string {
	return p.id
}

func (p *process) Kill(ctx context.Context, signal uint32, _ bool) error {
	_, err := p.shim.task.Kill(ctx, &task.KillRequest{
		Signal: signal,
		ID:     p.shim.ID(),
		ExecID: p.id,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (p *process) State(ctx context.Context) (runtime.State, error) {
	response, err := p.shim.task.State(ctx, &task.StateRequest{
		ID:     p.shim.ID(),
		ExecID: p.id,
	})
	if err != nil {
		if errors.Cause(err) != ttrpc.ErrClosed {
			return runtime.State{}, errdefs.FromGRPC(err)
		}
		return runtime.State{}, errdefs.ErrNotFound
	}
	var status runtime.Status
	switch response.Status {
	case tasktypes.StatusCreated:
		status = runtime.CreatedStatus
	case tasktypes.StatusRunning:
		status = runtime.RunningStatus
	case tasktypes.StatusStopped:
		status = runtime.StoppedStatus
	case tasktypes.StatusPaused:
		status = runtime.PausedStatus
	case tasktypes.StatusPausing:
		status = runtime.PausingStatus
	}
	return runtime.State{
		Pid:        response.Pid,
		Status:     status,
		Stdin:      response.Stdin,
		Stdout:     response.Stdout,
		Stderr:     response.Stderr,
		Terminal:   response.Terminal,
		ExitStatus: response.ExitStatus,
		ExitedAt:   response.ExitedAt,
	}, nil
}

// ResizePty changes the side of the process's PTY to the provided width and height
func (p *process) ResizePty(ctx context.Context, size runtime.ConsoleSize) error {
	_, err := p.shim.task.ResizePty(ctx, &task.ResizePtyRequest{
		ID:     p.shim.ID(),
		ExecID: p.id,
		Width:  size.Width,
		Height: size.Height,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

// CloseIO closes the provided IO pipe for the process
func (p *process) CloseIO(ctx context.Context) error {
	_, err := p.shim.task.CloseIO(ctx, &task.CloseIORequest{
		ID:     p.shim.ID(),
		ExecID: p.id,
		Stdin:  true,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

// Start the process
func (p *process) Start(ctx context.Context) error {
	_, err := p.shim.task.Start(ctx, &task.StartRequest{
		ID:     p.shim.ID(),
		ExecID: p.id,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

// Wait on the process to exit and return the exit status and timestamp
func (p *process) Wait(ctx context.Context) (*runtime.Exit, error) {
	response, err := p.shim.task.Wait(ctx, &task.WaitRequest{
		ID:     p.shim.ID(),
		ExecID: p.id,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return &runtime.Exit{
		Timestamp: response.ExitedAt,
		Status:    response.ExitStatus,
	}, nil
}

func (p *process) Delete(ctx context.Context) (*runtime.Exit, error) {
	response, err := p.shim.task.Delete(ctx, &task.DeleteRequest{
		ID:     p.shim.ID(),
		ExecID: p.id,
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return &runtime.Exit{
		Status:    response.ExitStatus,
		Timestamp: response.ExitedAt,
		Pid:       response.Pid,
	}, nil
}
