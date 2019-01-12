package controller

import (
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type noopMetric struct{}

func (noopMetric) Inc()            {}
func (noopMetric) Dec()            {}
func (noopMetric) Observe(float64) {}
func (noopMetric) Set(float64)     {}

type noopWorkqueueMetricsProvider struct{}

func (noopWorkqueueMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	return noopMetric{}
}

func (noopWorkqueueMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	return noopMetric{}
}

func (noopWorkqueueMetricsProvider) NewLatencyMetric(name string) workqueue.SummaryMetric {
	return noopMetric{}
}

func (noopWorkqueueMetricsProvider) NewWorkDurationMetric(name string) workqueue.SummaryMetric {
	return noopMetric{}
}

func (noopWorkqueueMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	return noopMetric{}
}

func (noopWorkqueueMetricsProvider) NewLongestRunningProcessorMicrosecondsMetric(name string) workqueue.SettableGaugeMetric {
	return noopMetric{}
}

func (noopWorkqueueMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return noopMetric{}
}

type noopCacheMetricsProvider struct{}

func (noopCacheMetricsProvider) NewListsMetric(name string) cache.CounterMetric { return noopMetric{} }
func (noopCacheMetricsProvider) NewListDurationMetric(name string) cache.SummaryMetric {
	return noopMetric{}
}
func (noopCacheMetricsProvider) NewItemsInListMetric(name string) cache.SummaryMetric {
	return noopMetric{}
}
func (noopCacheMetricsProvider) NewWatchesMetric(name string) cache.CounterMetric { return noopMetric{} }
func (noopCacheMetricsProvider) NewShortWatchesMetric(name string) cache.CounterMetric {
	return noopMetric{}
}
func (noopCacheMetricsProvider) NewWatchDurationMetric(name string) cache.SummaryMetric {
	return noopMetric{}
}
func (noopCacheMetricsProvider) NewItemsInWatchMetric(name string) cache.SummaryMetric {
	return noopMetric{}
}
func (noopCacheMetricsProvider) NewLastResourceVersionMetric(name string) cache.GaugeMetric {
	return noopMetric{}
}

func DisableAllControllerMetrics() {
	DisableControllerReflectorMetrics()
	DisableControllerWorkqueuMetrics()
}

func DisableControllerWorkqueuMetrics() {
	workqueue.SetProvider(noopWorkqueueMetricsProvider{})
}

func DisableControllerReflectorMetrics() {
	cache.SetReflectorMetricsProvider(noopCacheMetricsProvider{})
}
