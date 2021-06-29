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
	"github.com/containerd/cgroups"
	v1 "github.com/containerd/containerd/metrics/cgroups/v1"
	v2 "github.com/containerd/containerd/metrics/cgroups/v2"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/plugin"
	"github.com/containerd/containerd/runtime"
	metrics "github.com/docker/go-metrics"
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
	var (
		tm  runtime.TaskMonitor
		err error
	)
	if cgroups.Mode() == cgroups.Unified {
		tm, err = v2.NewTaskMonitor(ic.Context, ic.Events, ns)
	} else {
		tm, err = v1.NewTaskMonitor(ic.Context, ic.Events, ns)
	}
	if err != nil {
		return nil, err
	}
	if ns != nil {
		metrics.Register(ns)
	}
	ic.Meta.Platforms = append(ic.Meta.Platforms, platforms.DefaultSpec())
	return tm, nil
}
