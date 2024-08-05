package cluster

// A managed database is one whose lifecycle we control - initializing the cluster, adding/removing members, taking snapshots, etc.
// This is currently just used for the embedded etcd datastore. Kine and other external etcd clusters are NOT considered managed.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/nodepassword"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
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
	if c.managedDB == nil {
		return nil
	}
	rebootstrap := func() error {
		return c.storageBootstrap(ctx)
	}

	resetDone, err := c.managedDB.IsReset()
	if err != nil {
		return err
	}

	if c.config.ClusterReset {
		// If we're restoring from a snapshot, don't check the reset-flag - just reset and restore.
		if c.config.ClusterResetRestorePath != "" {
			return c.managedDB.Reset(ctx, rebootstrap)
		}

		// If the reset-flag doesn't exist, reset. This will create the reset-flag if it succeeds.
		if !resetDone {
			return c.managedDB.Reset(ctx, rebootstrap)
		}

		// The reset-flag exists, ask the user to remove it if they want to reset again.
		return fmt.Errorf("Managed etcd cluster membership was previously reset, please remove the cluster-reset flag and start %s normally. "+
			"If you need to perform another cluster reset, you must first manually delete the file at %s", version.Program, c.managedDB.ResetFile())
	}

	if resetDone {
		// If the cluster was reset, we need to delete the node passwd secret in case the node
		// password from the previously restored snapshot differs from the current password on disk.
		c.config.Runtime.ClusterControllerStarts["node-password-secret-cleanup"] = c.deleteNodePasswdSecret
	}

	// Starting the managed database will clear the reset-flag if set
	return c.managedDB.Start(ctx, c.clientAccessInfo)
}

// registerDBHandlers registers managed-datastore-specific callbacks, and installs additional HTTP route handlers.
// Note that for etcd, controllers only run on nodes with a local apiserver, in order to provide stable external
// management of etcd cluster membership without being disrupted when a member is removed from the cluster.
func (c *Cluster) registerDBHandlers(handler http.Handler) (http.Handler, error) {
	if c.managedDB == nil {
		return handler, nil
	}

	return c.managedDB.Register(handler)
}

// assignManagedDriver assigns a driver based on a number of different configuration variables.
// If a driver has been initialized it is used.
// If no specific endpoint has been requested and creating or joining has been requested,
// we use the default driver.
// If none of the above are true, no managed driver is assigned.
func (c *Cluster) assignManagedDriver(ctx context.Context) error {
	// Check all managed drivers for an initialized database on disk; use one if found
	for _, driver := range managed.Registered() {
		if err := driver.SetControlConfig(c.config); err != nil {
			return err
		}
		if ok, err := driver.IsInitialized(); err != nil {
			return err
		} else if ok {
			c.managedDB = driver
			return nil
		}
	}

	// If we have been asked to initialize or join a cluster, do so using the default managed database.
	if c.config.Datastore.Endpoint == "" && (c.config.ClusterInit || (c.config.Token != "" && c.config.JoinURL != "")) {
		c.managedDB = managed.Default()
	}

	return nil
}

// setupEtcdProxy starts a goroutine to periodically update the etcd proxy with the current list of
// cluster client URLs, as retrieved from etcd.
func (c *Cluster) setupEtcdProxy(ctx context.Context, etcdProxy etcd.Proxy) {
	if c.managedDB == nil {
		return
	}
	// We use Poll here instead of Until because we want to wait the interval before running the function.
	go wait.PollUntilContextCancel(ctx, 30*time.Second, false, func(ctx context.Context) (bool, error) {
		clientURLs, err := c.managedDB.GetMembersClientURLs(ctx)
		if err != nil {
			logrus.Warnf("Failed to get etcd ClientURLs: %v", err)
			return false, nil
		}
		// client URLs are a full URI, but the proxy only wants host:port
		for i, c := range clientURLs {
			u, err := url.Parse(c)
			if err != nil {
				logrus.Warnf("Failed to parse etcd ClientURL: %v", err)
				return false, nil
			}
			clientURLs[i] = u.Host
		}
		etcdProxy.Update(clientURLs)
		return false, nil
	})
}

// deleteNodePasswdSecret wipes out the node password secret after restoration
func (c *Cluster) deleteNodePasswdSecret(ctx context.Context) {
	nodeName := os.Getenv("NODE_NAME")
	secretsClient := c.config.Runtime.Core.Core().V1().Secret()
	if err := nodepassword.Delete(secretsClient, nodeName); err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Debugf("Node password secret is not found for node %s", nodeName)
			return
		}
		logrus.Warnf("failed to delete old node password secret: %v", err)
	}
}
