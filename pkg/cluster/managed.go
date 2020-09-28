package cluster

// A managed database is one whose lifecycle we control - initializing the cluster, adding/removing members, taking snapshots, etc.
// This is currently just used for the embedded etcd datastore. Kine and other external etcd clusters are NOT considered managed.

import (
	"context"
	"net/http"
	"time"

	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/sirupsen/logrus"
)

// testClusterDB returns a channel that will be closed when the datastore connection is available.
// The datastore is tested for readiness every 5 seconds until the test succeeds.
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

// start starts the database, unless a cluster reset has been requested, in which case
// it does that instead.
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

// assignManagedDriver checks to see if any managed databases are already configured or should be created/joined.
// If a driver has been initialized it is used, otherwise we create or join a cluster using the default driver.
func (c *Cluster) assignManagedDriver(ctx context.Context) error {
	// Check all managed drivers for an initialized database on disk; use one if found
	for _, driver := range managed.Registered() {
		if ok, err := driver.IsInitialized(ctx, c.config); err != nil {
			return err
		} else if ok {
			c.managedDB = driver
			return nil
		}
	}

	// If we have been asked to initialize or join a cluster, do so using the default managed database.
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
