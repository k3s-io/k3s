package cluster

import (
	"net/http"
)

// getHandler returns a basic request handler that processes requests through
// the cluster's request router chain.
func (c *Cluster) getHandler(handler http.Handler) (http.Handler, error) {
	next := c.router()

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		handler.ServeHTTP(rw, req)
		next.ServeHTTP(rw, req)
	}), nil
}

// router is a stub request router that returns a Service Unavailable response
// if no additional handlers are available.
func (c *Cluster) router() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if c.config.Runtime.Handler == nil {
			http.Error(rw, "starting", http.StatusServiceUnavailable)
			return
		}

		c.config.Runtime.Handler.ServeHTTP(rw, req)
	})
}
