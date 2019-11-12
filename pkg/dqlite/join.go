package dqlite

import (
	"context"

	"github.com/canonical/go-dqlite/client"
	"github.com/sirupsen/logrus"
)

func (d *DQLite) Test(ctx context.Context) error {
	var ips []string
	peers, err := d.NodeStore.Get(ctx)
	if err != nil {
		return err
	}

	for _, peer := range peers {
		ips = append(ips, peer.Address)
	}

	logrus.Infof("Testing connection to peers %v", ips)
	return d.Join(ctx, nil)
}

func (d *DQLite) Join(ctx context.Context, nodes []client.NodeInfo) error {
	if len(nodes) > 0 {
		if err := d.NodeStore.Set(ctx, nodes); err != nil {
			return err
		}
	}

	client, err := client.FindLeader(ctx, d.NodeStore, d.clientOpts...)
	if err != nil {
		return err
	}
	defer client.Close()

	current, err := client.Cluster(ctx)
	if err != nil {
		return err
	}

	for _, testNode := range current {
		if testNode.Address == d.NodeInfo.Address {
			return nil
		}
	}

	logrus.Infof("Joining dqlite cluster as address=%s, id=%d")
	return client.Add(ctx, d.NodeInfo)
}
