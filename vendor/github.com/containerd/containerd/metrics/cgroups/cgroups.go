// +build linux

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

package cgroups

import (
	"context"

	"github.com/containerd/cgroups"
	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/runtime/v1/linux"
	metrics "github.com/docker/go-metrics"
	"github.com/sirupsen/logrus"
)

// Config for the cgroups monitor
type Config struct {
	NoPrometheus bool `toml:"no_prometheus"`
}

func init() {
	plugin.Register(&plugin.Registration{
		Type:   plugin.TaskMonitorPlugin,
		ID:     "cgroups",
		InitFn: New,
		Config: &Config{},
	})
}

// New returns a new cgroups monitor
func New(ic *plugin.InitContext) (interface{}, error) {
	var ns *metrics.Namespace
	config := ic.Config.(*Config)
	if !config.NoPrometheus {
		ns = metrics.NewNamespace("container", "", nil)
	}
	collector := newCollector(ns)
	oom, err := newOOMCollector(ns)
	if err != nil {
		return nil, err
	}
	if ns != nil {
		metrics.Register(ns)
	}
	ic.Meta.Platforms = append(ic.Meta.Platforms, platforms.DefaultSpec())
	return &cgroupsMonitor{
		collector: collector,
		oom:       oom,
		context:   ic.Context,
		publisher: ic.Events,
	}, nil
}

type cgroupsMonitor struct {
	collector *collector
	oom       *oomCollector
	context   context.Context
	publisher events.Publisher
}

func (m *cgroupsMonitor) Monitor(c runtime.Task) error {
	if err := m.collector.Add(c); err != nil {
		return err
	}
	t, ok := c.(*linux.Task)
	if !ok {
		return nil
	}
	cg, err := t.Cgroup()
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}
	err = m.oom.Add(c.ID(), c.Namespace(), cg, m.trigger)
	if err == cgroups.ErrMemoryNotSupported {
		logrus.WithError(err).Warn("OOM monitoring failed")
		return nil
	}
	return err
}

func (m *cgroupsMonitor) Stop(c runtime.Task) error {
	m.collector.Remove(c)
	return nil
}

func (m *cgroupsMonitor) trigger(id, namespace string, cg cgroups.Cgroup) {
	ctx := namespaces.WithNamespace(m.context, namespace)
	if err := m.publisher.Publish(ctx, runtime.TaskOOMEventTopic, &eventstypes.TaskOOM{
		ContainerID: id,
	}); err != nil {
		log.G(m.context).WithError(err).Error("post OOM event")
	}
}
