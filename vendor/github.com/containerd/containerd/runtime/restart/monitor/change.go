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

package monitor

import (
	"context"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
)

type stopChange struct {
	container containerd.Container
}

func (s *stopChange) apply(ctx context.Context, client *containerd.Client) error {
	return killTask(ctx, s.container)
}

type startChange struct {
	container containerd.Container
	logPath   string
}

func (s *startChange) apply(ctx context.Context, client *containerd.Client) error {
	log := cio.NullIO
	if s.logPath != "" {
		log = cio.LogFile(s.logPath)
	}
	killTask(ctx, s.container)
	task, err := s.container.NewTask(ctx, log)
	if err != nil {
		return err
	}
	return task.Start(ctx)
}

func killTask(ctx context.Context, container containerd.Container) error {
	task, err := container.Task(ctx, nil)
	if err == nil {
		wait, err := task.Wait(ctx)
		if err != nil {
			if _, derr := task.Delete(ctx); derr == nil {
				return nil
			}
			return err
		}
		if err := task.Kill(ctx, syscall.SIGKILL, containerd.WithKillAll); err != nil {
			if _, derr := task.Delete(ctx); derr == nil {
				return nil
			}
			return err
		}
		<-wait
		if _, err := task.Delete(ctx); err != nil {
			return err
		}
	}
	return nil
}
