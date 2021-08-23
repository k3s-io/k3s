/*
Copyright 2019 The Kubernetes Authors.

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

package collectors

import (
	"time"

	"k8s.io/component-base/metrics"
	"k8s.io/klog/v2"
	summary "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
	"k8s.io/kubernetes/pkg/kubelet/server/stats"
)

var (
	nodeCPUUsageDesc = metrics.NewDesc("node_cpu_usage_seconds_total",
		"Cumulative cpu time consumed by the node in core-seconds",
		nil,
		nil,
		metrics.ALPHA,
		"")

	nodeMemoryUsageDesc = metrics.NewDesc("node_memory_working_set_bytes",
		"Current working set of the node in bytes",
		nil,
		nil,
		metrics.ALPHA,
		"")

	containerCPUUsageDesc = metrics.NewDesc("container_cpu_usage_seconds_total",
		"Cumulative cpu time consumed by the container in core-seconds",
		[]string{"container", "pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")

	containerMemoryUsageDesc = metrics.NewDesc("container_memory_working_set_bytes",
		"Current working set of the container in bytes",
		[]string{"container", "pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")

	podCPUUsageDesc = metrics.NewDesc("pod_cpu_usage_seconds_total",
		"Cumulative cpu time consumed by the pod in core-seconds",
		[]string{"pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")

	podMemoryUsageDesc = metrics.NewDesc("pod_memory_working_set_bytes",
		"Current working set of the pod in bytes",
		[]string{"pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")

	resourceScrapeResultDesc = metrics.NewDesc("scrape_error",
		"1 if there was an error while getting container metrics, 0 otherwise",
		nil,
		nil,
		metrics.ALPHA,
		"")

	containerStartTimeDesc = metrics.NewDesc("container_start_time_seconds",
		"Start time of the container since unix epoch in seconds",
		[]string{"container", "pod", "namespace"},
		nil,
		metrics.ALPHA,
		"")
)

// NewResourceMetricsCollector returns a metrics.StableCollector which exports resource metrics
func NewResourceMetricsCollector(provider stats.SummaryProvider) metrics.StableCollector {
	return &resourceMetricsCollector{
		provider: provider,
	}
}

type resourceMetricsCollector struct {
	metrics.BaseStableCollector

	provider stats.SummaryProvider
}

// Check if resourceMetricsCollector implements necessary interface
var _ metrics.StableCollector = &resourceMetricsCollector{}

// DescribeWithStability implements metrics.StableCollector
func (rc *resourceMetricsCollector) DescribeWithStability(ch chan<- *metrics.Desc) {
	ch <- nodeCPUUsageDesc
	ch <- nodeMemoryUsageDesc
	ch <- containerStartTimeDesc
	ch <- containerCPUUsageDesc
	ch <- containerMemoryUsageDesc
	ch <- podCPUUsageDesc
	ch <- podMemoryUsageDesc
	ch <- resourceScrapeResultDesc
}

// CollectWithStability implements metrics.StableCollector
// Since new containers are frequently created and removed, using the Gauge would
// leak metric collectors for containers or pods that no longer exist.  Instead, implement
// custom collector in a way that only collects metrics for active containers.
func (rc *resourceMetricsCollector) CollectWithStability(ch chan<- metrics.Metric) {
	var errorCount float64
	defer func() {
		ch <- metrics.NewLazyConstMetric(resourceScrapeResultDesc, metrics.GaugeValue, errorCount)
	}()
	statsSummary, err := rc.provider.GetCPUAndMemoryStats()
	if err != nil {
		errorCount = 1
		klog.ErrorS(err, "Error getting summary for resourceMetric prometheus endpoint")
		return
	}

	rc.collectNodeCPUMetrics(ch, statsSummary.Node)
	rc.collectNodeMemoryMetrics(ch, statsSummary.Node)

	for _, pod := range statsSummary.Pods {
		for _, container := range pod.Containers {
			rc.collectContainerStartTime(ch, pod, container)
			rc.collectContainerCPUMetrics(ch, pod, container)
			rc.collectContainerMemoryMetrics(ch, pod, container)
		}
		rc.collectPodCPUMetrics(ch, pod)
		rc.collectPodMemoryMetrics(ch, pod)
	}
}

func (rc *resourceMetricsCollector) collectNodeCPUMetrics(ch chan<- metrics.Metric, s summary.NodeStats) {
	if s.CPU == nil {
		return
	}

	ch <- metrics.NewLazyMetricWithTimestamp(s.CPU.Time.Time,
		metrics.NewLazyConstMetric(nodeCPUUsageDesc, metrics.CounterValue, float64(*s.CPU.UsageCoreNanoSeconds)/float64(time.Second)))
}

func (rc *resourceMetricsCollector) collectNodeMemoryMetrics(ch chan<- metrics.Metric, s summary.NodeStats) {
	if s.Memory == nil {
		return
	}

	ch <- metrics.NewLazyMetricWithTimestamp(s.Memory.Time.Time,
		metrics.NewLazyConstMetric(nodeMemoryUsageDesc, metrics.GaugeValue, float64(*s.Memory.WorkingSetBytes)))
}

func (rc *resourceMetricsCollector) collectContainerStartTime(ch chan<- metrics.Metric, pod summary.PodStats, s summary.ContainerStats) {
	if s.StartTime.Unix() == 0 {
		return
	}

	ch <- metrics.NewLazyMetricWithTimestamp(s.StartTime.Time,
		metrics.NewLazyConstMetric(containerStartTimeDesc, metrics.GaugeValue, float64(s.StartTime.UnixNano())/float64(time.Second), s.Name, pod.PodRef.Name, pod.PodRef.Namespace))
}

func (rc *resourceMetricsCollector) collectContainerCPUMetrics(ch chan<- metrics.Metric, pod summary.PodStats, s summary.ContainerStats) {
	if s.CPU == nil {
		return
	}

	ch <- metrics.NewLazyMetricWithTimestamp(s.CPU.Time.Time,
		metrics.NewLazyConstMetric(containerCPUUsageDesc, metrics.CounterValue,
			float64(*s.CPU.UsageCoreNanoSeconds)/float64(time.Second), s.Name, pod.PodRef.Name, pod.PodRef.Namespace))
}

func (rc *resourceMetricsCollector) collectContainerMemoryMetrics(ch chan<- metrics.Metric, pod summary.PodStats, s summary.ContainerStats) {
	if s.Memory == nil {
		return
	}

	ch <- metrics.NewLazyMetricWithTimestamp(s.Memory.Time.Time,
		metrics.NewLazyConstMetric(containerMemoryUsageDesc, metrics.GaugeValue,
			float64(*s.Memory.WorkingSetBytes), s.Name, pod.PodRef.Name, pod.PodRef.Namespace))
}

func (rc *resourceMetricsCollector) collectPodCPUMetrics(ch chan<- metrics.Metric, pod summary.PodStats) {
	if pod.CPU == nil {
		return
	}

	ch <- metrics.NewLazyMetricWithTimestamp(pod.CPU.Time.Time,
		metrics.NewLazyConstMetric(podCPUUsageDesc, metrics.CounterValue,
			float64(*pod.CPU.UsageCoreNanoSeconds)/float64(time.Second), pod.PodRef.Name, pod.PodRef.Namespace))
}

func (rc *resourceMetricsCollector) collectPodMemoryMetrics(ch chan<- metrics.Metric, pod summary.PodStats) {
	if pod.Memory == nil {
		return
	}

	ch <- metrics.NewLazyMetricWithTimestamp(pod.Memory.Time.Time,
		metrics.NewLazyConstMetric(podMemoryUsageDesc, metrics.GaugeValue,
			float64(*pod.Memory.WorkingSetBytes), pod.PodRef.Name, pod.PodRef.Namespace))
}
