package cluster

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/kine/pkg/client"
	"github.com/rancher/kine/pkg/endpoint"
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
	storageClient    client.Client
}

func (c *Cluster) Start(ctx context.Context) (<-chan struct{}, error) {
	if err := c.initClusterAndHTTPS(ctx); err != nil {
		return nil, errors.Wrap(err, "start cluster and https")
	}

	if err := c.start(ctx); err != nil {
		return nil, errors.Wrap(err, "start cluster and https")
	}

	ready, err := c.testClusterDB(ctx)
	if err != nil {
		return nil, err
	}

	if c.saveBootstrap {
		if err := c.save(ctx); err != nil {
			return nil, err
		}
	}

	if c.shouldBootstrap {
		if err := c.bootstrapped(); err != nil {
			return nil, err
		}
	}

	return ready, c.startStorage(ctx)
}

func (c *Cluster) startStorage(ctx context.Context) error {
	if c.storageStarted {
		return nil
	}
	c.storageStarted = true

	etcdConfig, err := endpoint.Listen(ctx, c.config.Datastore)
	if err != nil {
		return errors.Wrap(err, "creating storage endpoint")
	}

	c.etcdConfig = etcdConfig
	c.config.Datastore.Config = etcdConfig.TLSConfig
	c.config.Datastore.Endpoint = strings.Join(etcdConfig.Endpoints, ",")
	c.config.NoLeaderElect = !etcdConfig.LeaderElect
	return nil
}

func New(config *config.Control) *Cluster {
	return &Cluster{
		config:  config,
		runtime: config.Runtime,
	}
}
