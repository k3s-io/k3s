package dqlite

import (
	"context"
	"fmt"

	"github.com/canonical/go-dqlite/client"
	"github.com/sirupsen/logrus"
)

func (d *DQLite) Reset(ctx context.Context) error {
	dqClient, err := client.New(ctx, d.getBindAddress(), client.WithLogFunc(log()))
	if err != nil {
		return err
	}

	current, err := dqClient.Cluster(ctx)
	if err != nil {
		return err
	}

	// There's a chance our ID and the ID the server has doesn't match so find the ID
	var surviving []client.NodeInfo
	for _, testNode := range current {
		if testNode.Address == d.NodeInfo.Address && testNode.ID == d.NodeInfo.ID {
			surviving = append(surviving, testNode)
			continue
		}
		if err := dqClient.Remove(ctx, testNode.ID); err != nil {
			return err
		}
	}

	if len(surviving) != 1 {
		return fmt.Errorf("failed to find %s in the current node, can not reset", d.NodeInfo.Address)
	}

	logrus.Infof("Resetting cluster to single master, please rejoin members")
	return d.node.Recover(surviving)
}
