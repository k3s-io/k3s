package etcd

import (
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
)

var (
	snapshotSaveCount = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_etcd_snapshot_save_duration_seconds",
		Help:    "Total time in seconds taken to complete the etcd snapshot process, labeled by success/failure status.",
		Buckets: metrics.ExponentialBuckets(0.008, 2, 15),
	}, []string{"status"})

	snapshotSaveLocalCount = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_etcd_snapshot_save_local_duration_seconds",
		Help:    "Total time in seconds taken to save a local snapshot file, labeled by success/failure status.",
		Buckets: metrics.ExponentialBuckets(0.008, 2, 15),
	}, []string{"status"})

	snapshotSaveS3Count = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_etcd_snapshot_save_s3_duration_seconds",
		Help:    "Total time in seconds taken to upload a snapshot file to S3, labeled by success/failure status.",
		Buckets: metrics.ExponentialBuckets(0.008, 2, 15),
	}, []string{"status"})

	snapshotReconcileCount = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_etcd_snapshot_reconcile_duration_seconds",
		Help:    "Total time in seconds taken to sync the list of etcd snapshots, labeled by success/failure status.",
		Buckets: metrics.ExponentialBuckets(0.008, 2, 15),
	}, []string{"status"})

	snapshotReconcileLocalCount = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_etcd_snapshot_reconcile_local_duration_seconds",
		Help:    "Total time in seconds taken to list local snapshot files, labeled by success/failure status.",
		Buckets: metrics.ExponentialBuckets(0.008, 2, 15),
	}, []string{"status"})

	snapshotReconcileS3Count = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_etcd_snapshot_reconcile_s3_duration_seconds",
		Help:    "Total time in seconds taken to list S3 snapshot files, labeled by success/failure status.",
		Buckets: metrics.ExponentialBuckets(0.008, 2, 15),
	}, []string{"status"})
)

// MustRegister registers etcd snapshot metrics
func MustRegister(registerer prometheus.Registerer) {
	registerer.MustRegister(
		snapshotSaveCount,
		snapshotSaveLocalCount,
		snapshotSaveS3Count,
		snapshotReconcileCount,
		snapshotReconcileLocalCount,
		snapshotReconcileS3Count,
	)
}
