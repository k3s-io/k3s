package etcd

import (
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
)

var (
	snapshotResourceCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: version.Program + "_etcd_snapshot_resource_count",
		Help: "Count of ETCDSnapshotFile resources by location",
	}, []string{"location"})

	snapshotSaveCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: version.Program + "_etcd_snapshot_save_count",
		Help: "Count of etcd snapshot save operations by status",
	}, []string{"status"})

	snapshotSaveTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_etcd_snapshot_save_duration_seconds",
		Help:    "Time taken to save an etcd snapshot to disk",
		Buckets: metrics.ExponentialBuckets(0.001, 2, 15),
	})

	snapshotS3Count = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: version.Program + "_etcd_snapshot_s3_count",
		Help: "Count of etcd snapshot s3 upload operations by status",
	}, []string{"status"})

	snapshotS3Time = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_etcd_snapshot_s3_duration_seconds",
		Help:    "Time taken to upload an etcd snapshot to S3",
		Buckets: metrics.ExponentialBuckets(0.001, 2, 15),
	})
)
