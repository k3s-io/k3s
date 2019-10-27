package cluster

import (
	"net/http"
)

func (c *Cluster) getHandler(handler http.Handler) (http.Handler, error) {
	next := c.router()

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		handler.ServeHTTP(rw, req)
		next.ServeHTTP(rw, req)
	}), nil
}

func (c *Cluster) router() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if c.runtime.Handler == nil {
			http.Error(rw, "starting", http.StatusServiceUnavailable)
			return
		}

		c.runtime.Handler.ServeHTTP(rw, req)
	})
}
