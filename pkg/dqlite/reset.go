package dqlite

import (
	"context"

	"github.com/canonical/go-dqlite/client"
	"github.com/sirupsen/logrus"
)

func (d *DQLite) Reset(ctx context.Context) error {
	logrus.Infof("Resetting cluster to single master")
	return d.node.Recover([]client.NodeInfo{
		d.NodeInfo,
	})
}
