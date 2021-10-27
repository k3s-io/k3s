package cluster

import (
	"context"
	"net/url"
	"runtime"
	"strings"

	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/sirupsen/logrus"
)

type Cluster struct {
	clientAccessInfo *clientaccess.Info
	config           *config.Control
	runtime          *config.ControlRuntime
	managedDB        managed.Driver
	shouldBootstrap  bool
	storageStarted   bool
	etcdConfig       endpoint.ETCDConfig
	joining          bool
	saveBootstrap    bool
}

// Start creates the dynamic tls listener, http request handler,
// handles starting and writing/reading bootstrap data, and returns a channel
// that will be closed when datastore is ready. If embedded etcd is in use,
// a secondary call to Cluster.save is made.
func (c *Cluster) Start(ctx context.Context) (<-chan struct{}, error) {
	// Set up the dynamiclistener and http request handlers
	if err := c.initClusterAndHTTPS(ctx); err != nil {
		return nil, errors.Wrap(err, "init cluster datastore and https")
	}

	if c.config.DisableETCD {
		ready := make(chan struct{})
		defer close(ready)

		// try to get /db/info urls first before attempting to use join url
		clientURLs, _, err := etcd.ClientURLs(ctx, c.clientAccessInfo, c.config.PrivateIP)
		if err != nil {
			return nil, err
		}
		if len(clientURLs) < 1 {
			clientURL, err := url.Parse(c.config.JoinURL)
			if err != nil {
				return nil, err
			}
			clientURL.Host = clientURL.Hostname() + ":2379"
			clientURLs = append(clientURLs, clientURL.String())
		}
		etcdProxy, err := etcd.NewETCDProxy(ctx, true, c.config.DataDir, clientURLs[0])
		if err != nil {
			return nil, err
		}
		c.setupEtcdProxy(ctx, etcdProxy)

		// remove etcd member if it exists
		if err := c.managedDB.RemoveSelf(ctx); err != nil {
			logrus.Warnf("Failed to remove this node from etcd members")
		}

		return ready, nil
	}

	// start managed database (if necessary)
	if err := c.start(ctx); err != nil {
		return nil, errors.Wrap(err, "start managed database")
	}

	// get the wait channel for testing managed database readiness
	ready, err := c.testClusterDB(ctx)
	if err != nil {
		return nil, err
	}

	// if necessary, store bootstrap data to datastore
	if c.saveBootstrap {
		if err := c.save(ctx); err != nil {
			return nil, err
		}
	}

	// if necessary, record successful bootstrap
	if c.shouldBootstrap {
		if err := c.bootstrapped(); err != nil {
			return nil, err
		}
	}

	if err := c.startStorage(ctx); err != nil {
		return nil, err
	}

	// at this point, if etcd is in use, it's bootstrapping is complete
	// so save the bootstrap data. We will need for etcd to be up. If
	// the save call returns an error, we panic since subsequent etcd
	// snapshots will be empty.
	if c.managedDB != nil {
		go func() {
			for {
				select {
				case <-ready:
					if err := c.save(ctx); err != nil {
						panic(err)
					}

					if !c.config.EtcdDisableSnapshots {
						if err := c.managedDB.StoreSnapshotData(ctx); err != nil {
							logrus.Errorf("Failed to record snapshots for cluster: %v", err)
						}
					}

					return
				default:
					runtime.Gosched()
				}
			}
		}()
	}

	return ready, nil
}

// startStorage starts the kine listener and configures the endpoints, if necessary.
// This calls into the kine endpoint code, which sets up the database client
// and unix domain socket listener if using an external database. In the case of an etcd
// backend it just returns the user-provided etcd endpoints and tls config.
func (c *Cluster) startStorage(ctx context.Context) error {
	if c.storageStarted {
		return nil
	}
	c.storageStarted = true

	// start listening on the kine socket as an etcd endpoint, or return the external etcd endpoints
	etcdConfig, err := endpoint.Listen(ctx, c.config.Datastore)
	if err != nil {
		return errors.Wrap(err, "creating storage endpoint")
	}

	// Persist the returned etcd configuration. We decide if we're doing leader election for embedded controllers
	// based on what the kine wrapper tells us about the datastore. Single-node datastores like sqlite don't require
	// leader election, while basically all others (etcd, external database, etc) do since they allow multiple servers.
	c.etcdConfig = etcdConfig
	c.config.Datastore.Config = etcdConfig.TLSConfig
	c.config.Datastore.Endpoint = strings.Join(etcdConfig.Endpoints, ",")
	c.config.NoLeaderElect = !etcdConfig.LeaderElect
	return nil
}

// New creates an initial cluster using the provided configuration
func New(config *config.Control) *Cluster {
	return &Cluster{
		config:  config,
		runtime: config.Runtime,
	}
}
