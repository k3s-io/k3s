package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func ObserveWithStatus(vec *prometheus.HistogramVec, start time.Time, err error, labels ...string) {
	status := "success"
	if err != nil {
		status = "error"
	}
	labels = append(labels, status)
	vec.WithLabelValues(labels...).Observe(time.Since(start).Seconds())
}
