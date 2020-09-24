package cluster

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/kine/pkg/endpoint"
	"github.com/sirupsen/logrus"
)

func (c *Cluster) testClusterDB(ctx context.Context) (<-chan struct{}, error) {
	result := make(chan struct{})
	if c.managedDB == nil {
		close(result)
		return result, nil
	}

	go func() {
		defer close(result)
		for {
			if err := c.managedDB.Test(ctx, c.clientAccessInfo); err != nil {
				logrus.Infof("Failed to test data store connection: %v", err)
			} else {
				logrus.Info(c.managedDB.EndpointName() + " data store connection OK")
				return
			}

			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()

	return result, nil
}

func (c *Cluster) start(ctx context.Context) error {
	if c.managedDB == nil {
		return nil
	}

	if c.config.ClusterReset {
		return c.managedDB.Reset(ctx, c.clientAccessInfo)
	}

	return c.managedDB.Start(ctx, c.clientAccessInfo)
}

// initClusterDB registers routes for database info with the http request handler
func (c *Cluster) initClusterDB(ctx context.Context, handler http.Handler) (http.Handler, error) {
	if c.managedDB == nil {
		return handler, nil
	}

	return c.managedDB.Register(ctx, c.config, handler)
}

func (c *Cluster) assignManagedDriver(ctx context.Context) error {
	for _, driver := range managed.Registered() {
		if ok, err := driver.IsInitialized(ctx, c.config); err != nil {
			return err
		} else if ok {
			c.managedDB = driver
			return nil
		}
	}


	if c.config.Datastore.Endpoint == "" && (c.config.ClusterInit || (c.config.Token != "" && c.config.JoinURL != "")) {
		for _, driver := range managed.Registered() {
			if driver.EndpointName() == managed.Default() {
				c.managedDB = driver
				return nil
			}
		}
	}

	return nil
}
