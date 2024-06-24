package profile

import (
	"context"
	"errors"
	"net/http/pprof"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/agent/https"
	"github.com/k3s-io/k3s/pkg/daemons/config"
)

// DefaultProfiler the default instance of a performance profiling server
var DefaultProfiler = &Config{
	Router: func(context.Context, *config.Node) (*mux.Router, error) {
		return nil, errors.New("not implemented")
	},
}

// Config holds fields for the pprof listener
type Config struct {
	// Router will be called to add the pprof API handler to an existing router.
	Router https.RouterFunc
}

// Start starts binds the pprof API to an existing HTTP router.
func (c *Config) Start(ctx context.Context, nodeConfig *config.Node) error {
	mRouter, err := c.Router(ctx, nodeConfig)
	if err != nil {
		return err
	}
	mRouter.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mRouter.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mRouter.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mRouter.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mRouter.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
	return nil
}
