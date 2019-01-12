package metrics

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

const MetricsGenericControllerEnv = "NORMAN_GENERIC_CONTROLLER_METRICS"

var (
	genericControllerMetrics = false
	TotalHandlerExecution    = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "norman_generic_controller",
			Name:      "total_handler_execution",
			Help:      "Total Count of executing handler",
		},
		[]string{"name", "handlerName"},
	)

	TotalHandlerFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "norman_generic_controller",
			Name:      "total_handler_failure",
			Help:      "Total Count of handler failure",
		},
		[]string{"name", "handlerName", "key"},
	)
)

func init() {
	if os.Getenv(MetricsGenericControllerEnv) == "true" {
		genericControllerMetrics = true
	}
}

func IncTotalHandlerExecution(controllerName, handlerName string) {
	if genericControllerMetrics {
		TotalHandlerExecution.With(
			prometheus.Labels{
				"name":        controllerName,
				"handlerName": handlerName},
		).Inc()
	}
}

func IncTotalHandlerFailure(controllerName, handlerName, key string) {
	if genericControllerMetrics {
		TotalHandlerFailure.With(
			prometheus.Labels{
				"name":        controllerName,
				"handlerName": handlerName,
				"key":         key,
			},
		).Inc()
	}
}
