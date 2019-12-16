package client

import (
	"context"
	"fmt"
	"strconv"

	"github.com/canonical/go-dqlite/client"
	"github.com/canonical/go-dqlite/driver"
	controllerv1 "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	allKey      = "_all_"
	nodeID      = "cluster.k3s.cattle.io/node-id"
	nodeAddress = "cluster.k3s.cattle.io/node-address"
	master      = "node-role.kubernetes.io/master"
)

func Register(ctx context.Context, nodeName string, nodeInfo client.NodeInfo,
	nodeStore client.NodeStore, nodes controllerv1.NodeController, opts []client.Option) {
	h := &handler{
		nodeStore:      nodeStore,
		nodeController: nodes,
		nodeName:       nodeName,
		id:             strconv.FormatUint(nodeInfo.ID, 10),
		address:        nodeInfo.Address,
		ctx:            ctx,
		opts:           opts,
	}
	nodes.OnChange(ctx, "dqlite-client", h.sync)
	nodes.OnRemove(ctx, "dqlite-client", h.onRemove)
}

type handler struct {
	nodeStore      client.NodeStore
	nodeController controllerv1.NodeController
	nodeName       string
	id             string
	address        string
	ctx            context.Context
	opts           []client.Option
}

func (h *handler) sync(key string, node *v1.Node) (*v1.Node, error) {
	if key == allKey {
		return nil, h.updateNodeStore()
	}

	if node == nil {
		return nil, nil
	}

	if key == h.nodeName {
		return h.handleSelf(node)
	}

	if node.Labels[master] == "true" {
		h.nodeController.Enqueue(allKey)
	}

	return node, nil
}

func (h *handler) ensureExists(address string) error {
	c, err := client.FindLeader(h.ctx, h.nodeStore, h.opts...)
	if err == driver.ErrNoAvailableLeader {
		logrus.Fatalf("no dqlite leader found: %v", err)
	} else if err != nil {
		return err
	}
	defer c.Close()

	members, err := c.Cluster(h.ctx)
	if err != nil {
		return err
	}

	for _, member := range members {
		if member.Address == address {
			return nil
		}
	}

	logrus.Fatalf("Address %s is not member of the cluster", address)
	return nil
}

func (h *handler) handleSelf(node *v1.Node) (*v1.Node, error) {
	if node.Annotations[nodeID] == h.id && node.Annotations[nodeAddress] == h.address {
		return node, h.ensureExists(h.address)
	}

	node = node.DeepCopy()
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	node.Annotations[nodeID] = h.id
	node.Annotations[nodeAddress] = h.address

	return h.nodeController.Update(node)
}

func (h *handler) onRemove(key string, node *v1.Node) (*v1.Node, error) {
	address := node.Annotations[nodeAddress]
	if address == "" {
		return node, nil
	}
	return node, h.delete(address)
}

func (h *handler) delete(address string) error {
	c, err := client.FindLeader(h.ctx, h.nodeStore, h.opts...)
	if err != nil {
		return err
	}
	defer c.Close()

	members, err := c.Cluster(h.ctx)
	if err != nil {
		return err
	}

	for _, member := range members {
		if member.Address == address {
			logrus.Infof("Removing %s %d from dqlite", member.Address, member.ID)
			return c.Remove(h.ctx, member.ID)
		}
	}

	return nil
}

func (h *handler) updateNodeStore() error {
	nodes, err := h.nodeController.Cache().List(labels.SelectorFromSet(labels.Set{
		master: "true",
	}))
	if err != nil {
		return err
	}

	var (
		nodeInfos []client.NodeInfo
		seen      = map[string]bool{}
	)

	for _, node := range nodes {
		address, ok := node.Annotations[nodeAddress]
		if !ok {
			continue
		}

		nodeIDStr, ok := node.Annotations[nodeID]
		if !ok {
			continue
		}

		id, err := strconv.ParseUint(nodeIDStr, 10, 64)
		if err != nil {
			logrus.Errorf("invalid %s=%s, must be a number: %v", nodeID, nodeIDStr, err)
			continue
		}

		if !seen[address] {
			nodeInfos = append(nodeInfos, client.NodeInfo{
				ID:      id,
				Address: address,
			})
			seen[address] = true
		}
	}

	if len(nodeInfos) == 0 {
		return fmt.Errorf("not setting dqlient NodeStore len to 0")
	}

	return h.nodeStore.Set(h.ctx, nodeInfos)
}
