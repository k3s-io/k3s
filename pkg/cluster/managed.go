package cluster

// A managed database is one whose lifecycle we control - initializing the cluster, adding/removing members, taking snapshots, etc.
// This is currently just used for the embedded etcd datastore. Kine and other external etcd clusters are NOT considered managed.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/rancher/k3s/pkg/nodepassword"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			if err := c.managedDB.Test(ctx); err != nil {
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
	resetFile := etcd.ResetFile(c.config)
	if c.managedDB == nil {
		return nil
	}

	switch {
	case c.config.ClusterReset && c.config.ClusterResetRestorePath != "":
		rebootstrap := func() error {
			return c.storageBootstrap(ctx)
		}
		return c.managedDB.Reset(ctx, rebootstrap)
	case c.config.ClusterReset:
		if _, err := os.Stat(resetFile); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			rebootstrap := func() error {
				return c.storageBootstrap(ctx)
			}
			return c.managedDB.Reset(ctx, rebootstrap)
		}
		return fmt.Errorf("cluster-reset was successfully performed, please remove the cluster-reset flag and start %s normally, if you need to perform another cluster reset, you must first manually delete the %s file", version.Program, resetFile)
	}

	if _, err := os.Stat(resetFile); err == nil {
		// before removing reset file we need to delete the node passwd secret
		go c.deleteNodePasswdSecret(ctx)
	}
	// removing the reset file and ignore error if the file doesn't exist
	os.Remove(resetFile)

	return c.managedDB.Start(ctx, c.clientAccessInfo)
}

// initClusterDB registers routes for database info with the http request handler
func (c *Cluster) initClusterDB(ctx context.Context, handler http.Handler) (http.Handler, error) {
	if c.managedDB == nil {
		return handler, nil
	}

	if !strings.HasPrefix(c.config.Datastore.Endpoint, c.managedDB.EndpointName()+"://") {
		c.config.Datastore = endpoint.Config{
			Endpoint: c.managedDB.EndpointName(),
		}
	}

	return c.managedDB.Register(ctx, c.config, handler)
}

// assignManagedDriver assigns a driver based on a number of different configuration variables.
// If a driver has been initialized it is used.
// If the configured endpoint matches the name of a driver, that driver is used.
// If no specific endpoint has been requested and creating or joining has been requested,
// we use the default driver.
// If none of the above are true, no managed driver is assigned.
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

	// This is needed to allow downstreams to override driver selection logic by
	// setting ServerConfig.Datastore.Endpoint such that it will match a driver's EndpointName
	endpointType := strings.SplitN(c.config.Datastore.Endpoint, ":", 2)[0]
	for _, driver := range managed.Registered() {
		if endpointType == driver.EndpointName() {
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

// setupEtcdProxy
func (c *Cluster) setupEtcdProxy(ctx context.Context, etcdProxy etcd.Proxy) {
	if c.managedDB == nil {
		return
	}
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for range t.C {
			newAddresses, err := c.managedDB.GetMembersClientURLs(ctx)
			if err != nil {
				logrus.Warnf("failed to get etcd client URLs: %v", err)
				continue
			}
			// client URLs are a full URI, but the proxy only wants host:port
			var hosts []string
			for _, address := range newAddresses {
				u, err := url.Parse(address)
				if err != nil {
					logrus.Warnf("failed to parse etcd client URL: %v", err)
					continue
				}
				hosts = append(hosts, u.Host)
			}
			etcdProxy.Update(hosts)
		}
	}()
}

// deleteNodePasswdSecret wipes out the node password secret after restoration
func (c *Cluster) deleteNodePasswdSecret(ctx context.Context) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			logrus.Infof("waiting for node name to be set")
			continue
		}
		// the core factory may not yet be initialized so we
		// want to wait until it is so not to evoke a panic.
		if c.runtime.Core == nil {
			logrus.Infof("runtime is not yet initialized")
			continue
		}
		secretsClient := c.runtime.Core.Core().V1().Secret()
		if err := nodepassword.Delete(secretsClient, nodeName); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Debugf("node password secret is not found for node %s", nodeName)
				return
			}
			logrus.Warnf("failed to delete old node password secret: %v", err)
			continue
		}
		return
	}

}
