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
	"net/url"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
)

const (
	// StatusLabel sets the restart status label for a container
	StatusLabel = "containerd.io/restart.status"
	// LogURILabel sets the restart log uri label for a container
	LogURILabel = "containerd.io/restart.loguri"

	// LogPathLabel sets the restart log path label for a container
	//
	// Deprecated(in release 1.5): use LogURILabel
	LogPathLabel = "containerd.io/restart.logpath"
)

// WithLogURI sets the specified log uri for a container.
func WithLogURI(uri *url.URL) func(context.Context, *containerd.Client, *containers.Container) error {
	return WithLogURIString(uri.String())
}

// WithLogURIString sets the specified log uri string for a container.
func WithLogURIString(uriString string) func(context.Context, *containerd.Client, *containers.Container) error {
	return func(_ context.Context, _ *containerd.Client, c *containers.Container) error {
		ensureLabels(c)
		c.Labels[LogURILabel] = uriString
		return nil
	}
}

// WithBinaryLogURI sets the binary-type log uri for a container.
//
// Deprecated(in release 1.5): use WithLogURI
func WithBinaryLogURI(binary string, args map[string]string) func(context.Context, *containerd.Client, *containers.Container) error {
	uri, err := cio.LogURIGenerator("binary", binary, args)
	if err != nil {
		return func(context.Context, *containerd.Client, *containers.Container) error {
			return err
		}
	}
	return WithLogURI(uri)
}

// WithFileLogURI sets the file-type log uri for a container.
//
// Deprecated(in release 1.5): use WithLogURI
func WithFileLogURI(path string) func(context.Context, *containerd.Client, *containers.Container) error {
	uri, err := cio.LogURIGenerator("file", path, nil)
	if err != nil {
		return func(context.Context, *containerd.Client, *containers.Container) error {
			return err
		}
	}
	return WithLogURI(uri)
}

// WithLogPath sets the log path for a container
//
// Deprecated(in release 1.5): use WithLogURI with "file://<path>" URI.
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
	delete(c.Labels, LogURILabel)
	return nil
}

func ensureLabels(c *containers.Container) {
	if c.Labels == nil {
		c.Labels = make(map[string]string)
	}
}
