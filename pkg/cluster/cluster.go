package cluster

import (
	"context"

	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
)

type Cluster struct {
	token            string
	clientAccessInfo *clientaccess.Info
	config           *config.Control
	runtime          *config.ControlRuntime
	db               interface{}
}

func (c *Cluster) Start(ctx context.Context) error {
	join, err := c.shouldJoin()
	if err != nil {
		return err
	}

	if join {
		if err := c.join(); err != nil {
			return err
		}
	}

	if err := c.startClusterAndHTTPS(ctx); err != nil {
		return err
	}

	if join {
		if err := c.postJoin(ctx); err != nil {
			return err
		}
	}

	return c.joined()
}

func New(config *config.Control) *Cluster {
	return &Cluster{
		config:  config,
		runtime: config.Runtime,
	}
}
