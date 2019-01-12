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
	"fmt"
	"time"

	"github.com/containerd/containerd"
	containers "github.com/containerd/containerd/api/services/containers/v1"
	diff "github.com/containerd/containerd/api/services/diff/v1"
	images "github.com/containerd/containerd/api/services/images/v1"
	namespacesapi "github.com/containerd/containerd/api/services/namespaces/v1"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime/restart"
	"github.com/containerd/containerd/services"
	"github.com/containerd/containerd/snapshots"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// Config for the restart monitor
type Config struct {
	// Interval for how long to wait to check for state changes
	Interval duration `toml:"interval"`
}

func init() {
	plugin.Register(&plugin.Registration{
		Type: plugin.InternalPlugin,
		Requires: []plugin.Type{
			plugin.ServicePlugin,
		},
		ID: "restart",
		Config: &Config{
			Interval: duration{
				Duration: 10 * time.Second,
			},
		},
		InitFn: func(ic *plugin.InitContext) (interface{}, error) {
			opts, err := getServicesOpts(ic)
			if err != nil {
				return nil, err
			}
			client, err := containerd.New("", containerd.WithServices(opts...))
			if err != nil {
				return nil, err
			}
			m := &monitor{
				client: client,
			}
			go m.run(ic.Config.(*Config).Interval.Duration)
			return m, nil
		},
	})
}

// getServicesOpts get service options from plugin context.
func getServicesOpts(ic *plugin.InitContext) ([]containerd.ServicesOpt, error) {
	plugins, err := ic.GetByType(plugin.ServicePlugin)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get service plugin")
	}
	opts := []containerd.ServicesOpt{
		containerd.WithEventService(ic.Events),
	}
	for s, fn := range map[string]func(interface{}) containerd.ServicesOpt{
		services.ContentService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithContentStore(s.(content.Store))
		},
		services.ImagesService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithImageService(s.(images.ImagesClient))
		},
		services.SnapshotsService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithSnapshotters(s.(map[string]snapshots.Snapshotter))
		},
		services.ContainersService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithContainerService(s.(containers.ContainersClient))
		},
		services.TasksService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithTaskService(s.(tasks.TasksClient))
		},
		services.DiffService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithDiffService(s.(diff.DiffClient))
		},
		services.NamespacesService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithNamespaceService(s.(namespacesapi.NamespacesClient))
		},
		services.LeasesService: func(s interface{}) containerd.ServicesOpt {
			return containerd.WithLeasesService(s.(leases.Manager))
		},
	} {
		p := plugins[s]
		if p == nil {
			return nil, errors.Errorf("service %q not found", s)
		}
		i, err := p.Instance()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get instance of service %q", s)
		}
		if i == nil {
			return nil, errors.Errorf("instance of service %q not found", s)
		}
		opts = append(opts, fn(i))
	}
	return opts, nil
}

type change interface {
	apply(context.Context, *containerd.Client) error
}

type monitor struct {
	client *containerd.Client
}

func (m *monitor) run(interval time.Duration) {
	if interval == 0 {
		interval = 10 * time.Second
	}
	for {
		time.Sleep(interval)
		if err := m.reconcile(context.Background()); err != nil {
			logrus.WithError(err).Error("reconcile")
		}
	}
}

func (m *monitor) reconcile(ctx context.Context) error {
	ns, err := m.client.NamespaceService().List(ctx)
	if err != nil {
		return err
	}
	for _, name := range ns {
		ctx = namespaces.WithNamespace(ctx, name)
		changes, err := m.monitor(ctx)
		if err != nil {
			logrus.WithError(err).Error("monitor for changes")
			continue
		}
		for _, c := range changes {
			if err := c.apply(ctx, m.client); err != nil {
				logrus.WithError(err).Error("apply change")
			}
		}
	}
	return nil
}

func (m *monitor) monitor(ctx context.Context) ([]change, error) {
	containers, err := m.client.Containers(ctx, fmt.Sprintf("labels.%q", restart.StatusLabel))
	if err != nil {
		return nil, err
	}
	var changes []change
	for _, c := range containers {
		labels, err := c.Labels(ctx)
		if err != nil {
			return nil, err
		}
		desiredStatus := containerd.ProcessStatus(labels[restart.StatusLabel])
		if m.isSameStatus(ctx, desiredStatus, c) {
			continue
		}
		switch desiredStatus {
		case containerd.Running:
			changes = append(changes, &startChange{
				container: c,
				logPath:   labels[restart.LogPathLabel],
			})
		case containerd.Stopped:
			changes = append(changes, &stopChange{
				container: c,
			})
		}
	}
	return changes, nil
}

func (m *monitor) isSameStatus(ctx context.Context, desired containerd.ProcessStatus, container containerd.Container) bool {
	task, err := container.Task(ctx, nil)
	if err != nil {
		return desired == containerd.Stopped
	}
	state, err := task.Status(ctx)
	if err != nil {
		return desired == containerd.Stopped
	}
	return desired == state.Status
}
