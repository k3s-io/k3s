package etcd

import (
	"context"
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	v1 "k8s.io/api/core/v1"
)

func registerMemberHandlers(ctx context.Context, etcd *ETCD) {
	nodes := etcd.config.Runtime.Core.Core().V1().Node()
	e := &etcdMemberHandler{
		etcd:           etcd,
		nodeController: nodes,
		ctx:            ctx,
	}

	logrus.Infof("Starting managed etcd member removal controller")
	nodes.OnChange(ctx, "managed-etcd-controller", e.sync)
	nodes.OnRemove(ctx, "managed-etcd-controller", e.onRemove)
}

var (
	removalAnnotation         = "etcd." + version.Program + ".cattle.io/remove"
	removedNodeNameAnnotation = "etcd." + version.Program + ".cattle.io/removed-node-name"
)

type etcdMemberHandler struct {
	etcd           *ETCD
	nodeController controllerv1.NodeController
	ctx            context.Context
}

func (e *etcdMemberHandler) sync(key string, node *v1.Node) (*v1.Node, error) {
	if node == nil {
		return nil, nil
	}

	if _, ok := node.Labels[util.ETCDRoleLabelKey]; !ok {
		logrus.Debugf("Node %s was not labeled etcd node, skipping sync", key)
		return node, nil
	}

	node = node.DeepCopy()

	if removalRequested, ok := node.Annotations[removalAnnotation]; ok {
		// removal requires node name and address annotations; fail if either are not found
		name, ok := node.Annotations[NodeNameAnnotation]
		if !ok {
			return node, fmt.Errorf("node name annotation for node %s not found", key)
		}
		address, ok := node.Annotations[NodeAddressAnnotation]
		if !ok {
			return node, fmt.Errorf("node address annotation for node %s not found", key)
		}
		lf := logrus.Fields{"name": name, "address": address}

		// Check to see if the node was previously removed from the cluster
		if removed, ok := node.Annotations[removedNodeNameAnnotation]; ok {
			if removed != name {
				// If the current node name is not the same as the removed node name, clear the removal annotations,
				// as this indicates that the node has been re-added with a new name.
				logrus.WithFields(lf).Info("Resetting removed node flag as removed node name does not match current node name")
				delete(node.Annotations, removedNodeNameAnnotation)
				delete(node.Annotations, removalAnnotation)
				return e.nodeController.Update(node)
			}
			// Current node name matches removed node name; don't need to do anything
			return node, nil
		}

		if strings.ToLower(removalRequested) == "true" {
			// Removal requested, attempt to remove it from the cluster
			logrus.WithFields(lf).Info("Removing etcd member from cluster due to remove annotation")
			if err := e.etcd.RemovePeer(e.ctx, name, address, true); err != nil {
				// etcd will reject the removal if this is the only voting member; abort the removal by removing
				// the annotation if this is the case. The requesting controller can re-request removal by setting
				// the annotation again, once there are more cluster members.
				if errors.Is(err, rpctypes.ErrMemberNotEnoughStarted) {
					logrus.WithFields(lf).Errorf("etcd member removal rejected, clearing remove annotation: %v", err)
					delete(node.Annotations, removalAnnotation)
					return e.nodeController.Update(node)
				}
				return node, err
			}

			logrus.WithFields(lf).Info("etcd emember removed successfully")
			// Set the removed node name annotation and delete the etcd name and address annotations.
			// These will be re-set to their new value when the member rejoins the cluster.
			node.Annotations[removedNodeNameAnnotation] = name
			delete(node.Annotations, NodeNameAnnotation)
			delete(node.Annotations, NodeAddressAnnotation)
			return e.nodeController.Update(node)
		}
		// In the event that we had an unexpected removal annotation value, simply return.
		// Fallthrough to the non-op below.
	}
	// This is a non-op, as we don't have a deleted annotation to worry about.
	return node, nil
}

func (e *etcdMemberHandler) onRemove(key string, node *v1.Node) (*v1.Node, error) {
	if _, ok := node.Labels[util.ETCDRoleLabelKey]; !ok {
		logrus.Debugf("Node %s was not labeled etcd node, skipping etcd member removal", key)
		return node, nil
	}

	if removedNodeName, ok := node.Annotations[removedNodeNameAnnotation]; ok && len(removedNodeName) > 0 {
		logrus.Debugf("Node %s was already removed from the cluster, skipping etcd member removal", key)
		return node, nil
	}

	// removal requires node name and address annotations; fail if either are not found
	name, ok := node.Annotations[NodeNameAnnotation]
	if !ok {
		return node, fmt.Errorf("node name annotation for node %s not found", key)
	}
	address, ok := node.Annotations[NodeAddressAnnotation]
	if !ok {
		return node, fmt.Errorf("node address annotation for node %s not found", key)
	}
	lf := logrus.Fields{"name": name, "address": address}

	logrus.WithFields(lf).Info("Removing etcd member from cluster due to node delete")
	return node, e.etcd.RemovePeer(e.ctx, name, address, true)
}
