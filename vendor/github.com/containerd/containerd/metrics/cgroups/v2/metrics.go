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

package v2

import (
	"context"
	"fmt"
	"sync"

	"github.com/containerd/containerd/log"
	v2 "github.com/containerd/containerd/metrics/types/v2"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl"
	metrics "github.com/docker/go-metrics"
	"github.com/gogo/protobuf/types"
	"github.com/prometheus/client_golang/prometheus"
)

// Statable type that returns cgroup metrics
type Statable interface {
	ID() string
	Namespace() string
	Stats(context.Context) (*types.Any, error)
}

// NewCollector registers the collector with the provided namespace and returns it so
// that cgroups can be added for collection
func NewCollector(ns *metrics.Namespace) *Collector {
	if ns == nil {
		return &Collector{}
	}
	c := &Collector{
		ns:    ns,
		tasks: make(map[string]Statable),
	}
	c.metrics = append(c.metrics, pidMetrics...)
	c.metrics = append(c.metrics, cpuMetrics...)
	c.metrics = append(c.metrics, memoryMetrics...)
	c.metrics = append(c.metrics, ioMetrics...)
	c.storedMetrics = make(chan prometheus.Metric, 100*len(c.metrics))
	ns.Add(c)
	return c
}

func taskID(id, namespace string) string {
	return fmt.Sprintf("%s-%s", id, namespace)
}

// Collector provides the ability to collect container stats and export
// them in the prometheus format
type Collector struct {
	mu sync.RWMutex

	tasks         map[string]Statable
	ns            *metrics.Namespace
	metrics       []*metric
	storedMetrics chan prometheus.Metric
}

// Describe prometheus metrics
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range c.metrics {
		ch <- m.desc(c.ns)
	}
}

// Collect prometheus metrics
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	wg := &sync.WaitGroup{}
	for _, t := range c.tasks {
		wg.Add(1)
		go c.collect(t, ch, true, wg)
	}
storedLoop:
	for {
		// read stored metrics until the channel is flushed
		select {
		case m := <-c.storedMetrics:
			ch <- m
		default:
			break storedLoop
		}
	}
	c.mu.RUnlock()
	wg.Wait()
}

func (c *Collector) collect(t Statable, ch chan<- prometheus.Metric, block bool, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	ctx := namespaces.WithNamespace(context.Background(), t.Namespace())
	stats, err := t.Stats(ctx)
	if err != nil {
		log.L.WithError(err).Errorf("stat task %s", t.ID())
		return
	}
	data, err := typeurl.UnmarshalAny(stats)
	if err != nil {
		log.L.WithError(err).Errorf("unmarshal stats for %s", t.ID())
		return
	}
	s, ok := data.(*v2.Metrics)
	if !ok {
		log.L.WithError(err).Errorf("invalid metric type for %s", t.ID())
		return
	}
	for _, m := range c.metrics {
		m.collect(t.ID(), t.Namespace(), s, c.ns, ch, block)
	}
}

// Add adds the provided cgroup and id so that metrics are collected and exported
func (c *Collector) Add(t Statable) error {
	if c.ns == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	id := taskID(t.ID(), t.Namespace())
	if _, ok := c.tasks[id]; ok {
		return nil // requests to collect metrics should be idempotent
	}
	c.tasks[id] = t
	return nil
}

// Remove removes the provided cgroup by id from the collector
func (c *Collector) Remove(t Statable) {
	if c.ns == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tasks, taskID(t.ID(), t.Namespace()))
}

// RemoveAll statable items from the collector
func (c *Collector) RemoveAll() {
	if c.ns == nil {
		return
	}
	c.mu.Lock()
	c.tasks = make(map[string]Statable)
	c.mu.Unlock()
}
