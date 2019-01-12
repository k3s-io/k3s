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
	metrics "github.com/docker/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type value struct {
	v float64
	l []string
}

type metric struct {
	name   string
	help   string
	unit   metrics.Unit
	vt     prometheus.ValueType
	labels []string
	// getValues returns the value and labels for the data
	getValues func(stats *cgroups.Metrics) []value
}

func (m *metric) desc(ns *metrics.Namespace) *prometheus.Desc {
	// the namespace label is for containerd namespaces
	return ns.NewDesc(m.name, m.help, m.unit, append([]string{"container_id", "namespace"}, m.labels...)...)
}

func (m *metric) collect(id, namespace string, stats *cgroups.Metrics, ns *metrics.Namespace, ch chan<- prometheus.Metric, block bool) {
	values := m.getValues(stats)
	for _, v := range values {
		// block signals to block on the sending the metrics so none are missed
		if block {
			ch <- prometheus.MustNewConstMetric(m.desc(ns), m.vt, v.v, append([]string{id, namespace}, v.l...)...)
			continue
		}
		// non-blocking metrics can be dropped if the chan is full
		select {
		case ch <- prometheus.MustNewConstMetric(m.desc(ns), m.vt, v.v, append([]string{id, namespace}, v.l...)...):
		default:
		}
	}
}
