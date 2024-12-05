package loadbalancer

import (
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/component-base/metrics"
)

var (
	loadbalancerConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: version.Program + "_loadbalancer_server_connections",
		Help: "Count of current connections to loadbalancer server",
	}, []string{"name", "server"})

	loadbalancerState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: version.Program + "_loadbalancer_server_health",
		Help: "Current health value of loadbalancer server",
	}, []string{"name", "server"})

	loadbalancerDials = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    version.Program + "_loadbalancer_dial_duration_seconds",
		Help:    "Time taken to dial a connection to a backend server",
		Buckets: metrics.ExponentialBuckets(0.001, 2, 15),
	}, []string{"name", "status"})
)

// MustRegister registers loadbalancer metrics
func MustRegister(registerer prometheus.Registerer) {
	registerer.MustRegister(loadbalancerConnections, loadbalancerState, loadbalancerDials)
}
