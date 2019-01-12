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

// Package restart enables containers to have labels added and monitored to
// keep the container's task running if it is killed.
//
// Setting the StatusLabel on a container instructs the restart monitor to keep
// that container's task in a specific status.
// Setting the LogPathLabel on a container will setup the task's IO to be redirected
// to a log file when running a task within the restart manager.
//
// The restart labels can be cleared off of a container using the WithNoRestarts Opt.
//
// The restart monitor has one option in the containerd config under the [plugins.restart]
// section.  `interval = "10s" sets the reconcile interval that the restart monitor checks
// for task state and reconciles the desired status for that task.
package restart

import (
	"context"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/containers"
)

const (
	// StatusLabel sets the restart status label for a container
	StatusLabel = "containerd.io/restart.status"
	// LogPathLabel sets the restart log path label for a container
	LogPathLabel = "containerd.io/restart.logpath"
)

// WithLogPath sets the log path for a container
func WithLogPath(path string) func(context.Context, *containerd.Client, *containers.Container) error {
	return func(_ context.Context, _ *containerd.Client, c *containers.Container) error {
		ensureLabels(c)
		c.Labels[LogPathLabel] = path
		return nil
	}
}

// WithStatus sets the status for a container
func WithStatus(status containerd.ProcessStatus) func(context.Context, *containerd.Client, *containers.Container) error {
	return func(_ context.Context, _ *containerd.Client, c *containers.Container) error {
		ensureLabels(c)
		c.Labels[StatusLabel] = string(status)
		return nil
	}
}

// WithNoRestarts clears any restart information from the container
func WithNoRestarts(_ context.Context, _ *containerd.Client, c *containers.Container) error {
	if c.Labels == nil {
		return nil
	}
	delete(c.Labels, StatusLabel)
	delete(c.Labels, LogPathLabel)
	return nil
}

func ensureLabels(c *containers.Container) {
	if c.Labels == nil {
		c.Labels = make(map[string]string)
	}
}
