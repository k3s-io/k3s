package metrics

import (
	"context"
	"errors"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/agent/https"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/component-base/metrics/legacyregistry"
)

// DefaultRegisterer is the implementation of the
// prometheus Registerer interface that all metrics operations
// will use.
var DefaultRegisterer = legacyregistry.Registerer()

// DefaultGatherer  is the implementation of the
// prometheus Gatherere interface that all metrics operations
// will use.
var DefaultGatherer = legacyregistry.DefaultGatherer

// DefaultMetrics is the default instance of a Metrics server
var DefaultMetrics = &Config{
	Router: func(context.Context, *config.Node) (*mux.Router, error) {
		return nil, errors.New("not implemented")
	},
}

// Config holds fields for the metrics listener
type Config struct {
	// Router will be called to add the metrics API handler to an existing router.
	Router https.RouterFunc
}

// Start starts binds the metrics API to an existing HTTP router.
func (c *Config) Start(ctx context.Context, nodeConfig *config.Node) error {
	mRouter, err := c.Router(ctx, nodeConfig)
	if err != nil {
		return err
	}
	mRouter.Handle("/metrics", promhttp.HandlerFor(DefaultGatherer, promhttp.HandlerOpts{}))
	return nil
}
