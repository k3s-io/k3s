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
				logrus.Infof("Data store connection OK")
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

func (c *Cluster) initClusterDB(ctx context.Context, l net.Listener, handler http.Handler) (net.Listener, http.Handler, error) {
	if c.managedDB == nil {
		return l, handler, nil
	}

	if !strings.HasPrefix(c.config.Datastore.Endpoint, c.managedDB.EndpointName()+"://") {
		c.config.Datastore = endpoint.Config{
			Endpoint: c.managedDB.EndpointName(),
		}
	}

	return c.managedDB.Register(ctx, c.config, l, handler)
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

	endpointType := strings.SplitN(c.config.Datastore.Endpoint, ":", 2)[0]
	for _, driver := range managed.Registered() {
		if endpointType == driver.EndpointName() {
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
