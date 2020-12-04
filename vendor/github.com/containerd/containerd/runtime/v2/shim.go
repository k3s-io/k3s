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
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/api/types"
	tasktypes "github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/identifiers"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/timeout"
	"github.com/containerd/containerd/runtime"
	client "github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/ttrpc"
	ptypes "github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	loadTimeout     = "io.containerd.timeout.shim.load"
	cleanupTimeout  = "io.containerd.timeout.shim.cleanup"
	shutdownTimeout = "io.containerd.timeout.shim.shutdown"
)

func init() {
	timeout.Set(loadTimeout, 5*time.Second)
	timeout.Set(cleanupTimeout, 5*time.Second)
	timeout.Set(shutdownTimeout, 3*time.Second)
}

func loadAddress(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func loadShim(ctx context.Context, bundle *Bundle, events *exchange.Exchange, rt *runtime.TaskList, onClose func()) (_ *shim, err error) {
	address, err := loadAddress(filepath.Join(bundle.Path, "address"))
	if err != nil {
		return nil, err
	}
	conn, err := client.Connect(address, client.AnonReconnectDialer)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			conn.Close()
		}
	}()
	f, err := openShimLog(ctx, bundle, client.AnonReconnectDialer)
	if err != nil {
		return nil, errors.Wrap(err, "open shim log pipe")
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	// open the log pipe and block until the writer is ready
	// this helps with synchronization of the shim
	// copy the shim's logs to containerd's output
	go func() {
		defer f.Close()
		if _, err := io.Copy(os.Stderr, f); err != nil {
			// When using a multi-container shim the 2nd to Nth container in the
			// shim will not have a separate log pipe. Ignore the failure log
			// message here when the shim connect times out.
			if !errors.Is(err, os.ErrNotExist) {
				log.G(ctx).WithError(err).Error("copy shim log")
			}
		}
	}()

	client := ttrpc.NewClient(conn, ttrpc.WithOnClose(onClose))
	defer func() {
		if err != nil {
			client.Close()
		}
	}()
	s := &shim{
		client:  client,
		task:    task.NewTaskClient(client),
		bundle:  bundle,
		events:  events,
		rtTasks: rt,
	}
	ctx, cancel := timeout.WithContext(ctx, loadTimeout)
	defer cancel()
	if err := s.Connect(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func cleanupAfterDeadShim(ctx context.Context, id, ns string, events *exchange.Exchange, binaryCall *binary) {
	ctx = namespaces.WithNamespace(ctx, ns)
	ctx, cancel := timeout.WithContext(ctx, cleanupTimeout)
	defer cancel()

	log.G(ctx).WithFields(logrus.Fields{
		"id":        id,
		"namespace": ns,
	}).Warn("cleaning up after shim disconnected")
	response, err := binaryCall.Delete(ctx)
	if err != nil {
		log.G(ctx).WithError(err).WithFields(logrus.Fields{
			"id":        id,
			"namespace": ns,
		}).Warn("failed to clean up after shim disconnected")
	}

	var (
		pid        uint32
		exitStatus uint32
		exitedAt   time.Time
	)
	if response != nil {
		pid = response.Pid
		exitStatus = response.Status
		exitedAt = response.Timestamp
	} else {
		exitStatus = 255
		exitedAt = time.Now()
	}
	events.Publish(ctx, runtime.TaskExitEventTopic, &eventstypes.TaskExit{
		ContainerID: id,
		ID:          id,
		Pid:         pid,
		ExitStatus:  exitStatus,
		ExitedAt:    exitedAt,
	})

	events.Publish(ctx, runtime.TaskDeleteEventTopic, &eventstypes.TaskDelete{
		ContainerID: id,
		Pid:         pid,
		ExitStatus:  exitStatus,
		ExitedAt:    exitedAt,
	})
}

type shim struct {
	bundle  *Bundle
	client  *ttrpc.Client
	task    task.TaskService
	taskPid int
	events  *exchange.Exchange
	rtTasks *runtime.TaskList
}

func (s *shim) Connect(ctx context.Context) error {
	response, err := s.task.Connect(ctx, &task.ConnectRequest{
		ID: s.ID(),
	})
	if err != nil {
		return err
	}
	s.taskPid = int(response.TaskPid)
	return nil
}

func (s *shim) Shutdown(ctx context.Context) error {
	_, err := s.task.Shutdown(ctx, &task.ShutdownRequest{
		ID: s.ID(),
	})
	if err != nil && !errors.Is(err, ttrpc.ErrClosed) {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (s *shim) waitShutdown(ctx context.Context) error {
	ctx, cancel := timeout.WithContext(ctx, shutdownTimeout)
	defer cancel()
	return s.Shutdown(ctx)
}

// ID of the shim/task
func (s *shim) ID() string {
	return s.bundle.ID
}

// PID of the task
func (s *shim) PID() uint32 {
	return uint32(s.taskPid)
}

func (s *shim) Namespace() string {
	return s.bundle.Namespace
}

func (s *shim) Close() error {
	return s.client.Close()
}

func (s *shim) Delete(ctx context.Context) (*runtime.Exit, error) {
	response, shimErr := s.task.Delete(ctx, &task.DeleteRequest{
		ID: s.ID(),
	})
	if shimErr != nil {
		log.G(ctx).WithField("id", s.ID()).WithError(shimErr).Debug("failed to delete task")
		if !errors.Is(shimErr, ttrpc.ErrClosed) {
			shimErr = errdefs.FromGRPC(shimErr)
			if !errdefs.IsNotFound(shimErr) {
				return nil, shimErr
			}
		}
	}
	// remove self from the runtime task list
	// this seems dirty but it cleans up the API across runtimes, tasks, and the service
	s.rtTasks.Delete(ctx, s.ID())
	if err := s.waitShutdown(ctx); err != nil {
		log.G(ctx).WithField("id", s.ID()).WithError(err).Error("failed to shutdown shim")
	}
	s.Close()
	if err := s.bundle.Delete(); err != nil {
		log.G(ctx).WithField("id", s.ID()).WithError(err).Error("failed to delete bundle")
	}
	if shimErr != nil {
		return nil, shimErr
	}
	return &runtime.Exit{
		Status:    response.ExitStatus,
		Timestamp: response.ExitedAt,
		Pid:       response.Pid,
	}, nil
}

func (s *shim) Create(ctx context.Context, opts runtime.CreateOpts) (runtime.Task, error) {
	topts := opts.TaskOptions
	if topts == nil {
		topts = opts.RuntimeOptions
	}
	request := &task.CreateTaskRequest{
		ID:         s.ID(),
		Bundle:     s.bundle.Path,
		Stdin:      opts.IO.Stdin,
		Stdout:     opts.IO.Stdout,
		Stderr:     opts.IO.Stderr,
		Terminal:   opts.IO.Terminal,
		Checkpoint: opts.Checkpoint,
		Options:    topts,
	}
	for _, m := range opts.Rootfs {
		request.Rootfs = append(request.Rootfs, &types.Mount{
			Type:    m.Type,
			Source:  m.Source,
			Options: m.Options,
		})
	}
	response, err := s.task.Create(ctx, request)
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	s.taskPid = int(response.Pid)
	return s, nil
}

func (s *shim) Pause(ctx context.Context) error {
	if _, err := s.task.Pause(ctx, &task.PauseRequest{
		ID: s.ID(),
	}); err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (s *shim) Resume(ctx context.Context) error {
	if _, err := s.task.Resume(ctx, &task.ResumeRequest{
		ID: s.ID(),
	}); err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (s *shim) Start(ctx context.Context) error {
	response, err := s.task.Start(ctx, &task.StartRequest{
		ID: s.ID(),
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	s.taskPid = int(response.Pid)
	return nil
}

func (s *shim) Kill(ctx context.Context, signal uint32, all bool) error {
	if _, err := s.task.Kill(ctx, &task.KillRequest{
		ID:     s.ID(),
		Signal: signal,
		All:    all,
	}); err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (s *shim) Exec(ctx context.Context, id string, opts runtime.ExecOpts) (runtime.Process, error) {
	if err := identifiers.Validate(id); err != nil {
		return nil, errors.Wrapf(err, "invalid exec id %s", id)
	}
	request := &task.ExecProcessRequest{
		ID:       s.ID(),
		ExecID:   id,
		Stdin:    opts.IO.Stdin,
		Stdout:   opts.IO.Stdout,
		Stderr:   opts.IO.Stderr,
		Terminal: opts.IO.Terminal,
		Spec:     opts.Spec,
	}
	if _, err := s.task.Exec(ctx, request); err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return &process{
		id:   id,
		shim: s,
	}, nil
}

func (s *shim) Pids(ctx context.Context) ([]runtime.ProcessInfo, error) {
	resp, err := s.task.Pids(ctx, &task.PidsRequest{
		ID: s.ID(),
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	var processList []runtime.ProcessInfo
	for _, p := range resp.Processes {
		processList = append(processList, runtime.ProcessInfo{
			Pid:  p.Pid,
			Info: p.Info,
		})
	}
	return processList, nil
}

func (s *shim) ResizePty(ctx context.Context, size runtime.ConsoleSize) error {
	_, err := s.task.ResizePty(ctx, &task.ResizePtyRequest{
		ID:     s.ID(),
		Width:  size.Width,
		Height: size.Height,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (s *shim) CloseIO(ctx context.Context) error {
	_, err := s.task.CloseIO(ctx, &task.CloseIORequest{
		ID:    s.ID(),
		Stdin: true,
	})
	if err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (s *shim) Wait(ctx context.Context) (*runtime.Exit, error) {
	response, err := s.task.Wait(ctx, &task.WaitRequest{
		ID: s.ID(),
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return &runtime.Exit{
		Pid:       uint32(s.taskPid),
		Timestamp: response.ExitedAt,
		Status:    response.ExitStatus,
	}, nil
}

func (s *shim) Checkpoint(ctx context.Context, path string, options *ptypes.Any) error {
	request := &task.CheckpointTaskRequest{
		ID:      s.ID(),
		Path:    path,
		Options: options,
	}
	if _, err := s.task.Checkpoint(ctx, request); err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (s *shim) Update(ctx context.Context, resources *ptypes.Any) error {
	if _, err := s.task.Update(ctx, &task.UpdateTaskRequest{
		ID:        s.ID(),
		Resources: resources,
	}); err != nil {
		return errdefs.FromGRPC(err)
	}
	return nil
}

func (s *shim) Stats(ctx context.Context) (*ptypes.Any, error) {
	response, err := s.task.Stats(ctx, &task.StatsRequest{
		ID: s.ID(),
	})
	if err != nil {
		return nil, errdefs.FromGRPC(err)
	}
	return response.Stats, nil
}

func (s *shim) Process(ctx context.Context, id string) (runtime.Process, error) {
	p := &process{
		id:   id,
		shim: s,
	}
	if _, err := p.State(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *shim) State(ctx context.Context) (runtime.State, error) {
	response, err := s.task.State(ctx, &task.StateRequest{
		ID: s.ID(),
	})
	if err != nil {
		if !errors.Is(err, ttrpc.ErrClosed) {
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
