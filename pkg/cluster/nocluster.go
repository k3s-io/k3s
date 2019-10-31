// +build !dqlite

package cluster

import (
	"context"
	"net"
	"net/http"
)

func (c *Cluster) testClusterDB(ctx context.Context) error {
	return nil
}

func (c *Cluster) initClusterDB(ctx context.Context, l net.Listener, handler http.Handler) (net.Listener, http.Handler, error) {
	return l, handler, nil
}

func (c *Cluster) postJoin(ctx context.Context) error {
	return nil
}
