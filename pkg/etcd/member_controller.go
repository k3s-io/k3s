package etcd

import (
	"context"
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/pkg/version"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

func registerMemberHandlers(ctx context.Context, etcd *ETCD) {
	if etcd.config.DisableETCD {
		return
	}

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

	if _, ok := node.Labels[EtcdRoleLabel]; !ok {
		logrus.Debugf("Node %s was not labeled etcd node, skipping sync", key)
		return node, nil
	}

	node = node.DeepCopy()

	if removalRequested, ok := node.Annotations[removalAnnotation]; ok {
		if removed, ok := node.Annotations[removedNodeNameAnnotation]; ok {
			// check to see if removed is true. if it is, nothing to do.
			if currentNodeName, ok := node.Annotations[NodeNameAnnotation]; ok {
				if currentNodeName != removed {
					// If the current node name is not the same as the removed node name, reset the tainted annotation and removed node name
					logrus.Infof("Resetting removed node flag as removed node name (did not match current node name")
					delete(node.Annotations, removedNodeNameAnnotation)
					node.Annotations[removalAnnotation] = "false"
					return e.nodeController.Update(node)
				}
				// this is the case where the current node name matches the removed node name. We have already removed the
				// node, so no need to perform any action. Fallthrough to the non-op below.
			}
			// This is the edge case where the removed annotation exists, but there is not a current node name annotation.
			// This should be a non-op, as we can't remove the node anyway.
			logrus.Debugf("etcd member %s was already marked via annotations as removed", key)
			return node, nil
		}
		if strings.ToLower(removalRequested) == "true" {
			// remove the member.
			name, ok := node.Annotations[NodeNameAnnotation]
			if !ok {
				return node, fmt.Errorf("node name annotation for node %s not found", key)
			}
			address, ok := node.Annotations[NodeAddressAnnotation]
			if !ok {
				return node, fmt.Errorf("node address annotation for node %s not found", key)
			}

			logrus.Debugf("removing etcd member from cluster name: %s address: %s", name, address)
			if err := e.etcd.RemovePeer(e.ctx, name, address, true); err != nil {
				return node, err
			}
			logrus.Debugf("etcd member removal successful for name: %s address: %s", name, address)
			// Set the removed node name annotation and clean up the other etcd node annotations.
			// These will be set if the tombstone file is then created and the etcd member is re-added, to their new
			// respective values.
			node.Annotations[removedNodeNameAnnotation] = name
			delete(node.Annotations, NodeNameAnnotation)
			delete(node.Annotations, NodeAddressAnnotation)
			return e.nodeController.Update(node)
		}
		// In the event that we had an unexpected removal value, simply return.
		// Fallthrough to the non-op below.
	}
	// This is a non-op, as we don't have a tainted annotation to worry about.
	return node, nil
}

func (e *etcdMemberHandler) onRemove(key string, node *v1.Node) (*v1.Node, error) {
	if _, ok := node.Labels[EtcdRoleLabel]; !ok {
		logrus.Debugf("Node %s was not labeled etcd node, skipping etcd member removal", key)
		return node, nil
	}
	logrus.Infof("Removing etcd member %s from cluster", key)
	if removalRequested, ok := node.Annotations[removalAnnotation]; ok {
		if strings.ToLower(removalRequested) == "true" {
			if removedNodeName, ok := node.Annotations[removedNodeNameAnnotation]; ok {
				if len(removedNodeName) > 0 {
					// If we received a node to delete that has already been removed via annotation, it will be missing
					// the corresponding node name and address annotations.
					logrus.Infof("etcd member %s was already removed as member name %s via annotation from the cluster", key, removedNodeName)
					return node, nil
				}
			}
		}
	}
	name, ok := node.Annotations[NodeNameAnnotation]
	if !ok {
		return node, fmt.Errorf("node name annotation for node %s not found", key)
	}
	address, ok := node.Annotations[NodeAddressAnnotation]
	if !ok {
		return node, fmt.Errorf("node address annotation for node %s not found", key)
	}
	return node, e.etcd.RemovePeer(e.ctx, name, address, true)
}
