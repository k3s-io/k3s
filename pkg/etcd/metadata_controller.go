package etcd

import (
	"context"
	"os"
	"time"

	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

func RegisterMetadataHandlers(ctx context.Context, etcd *ETCD, nodes controllerv1.NodeController) {
	h := &metadataHandler{
		etcd:           etcd,
		nodeController: nodes,
		ctx:            ctx,
	}
	nodes.OnChange(ctx, "managed-etcd-metadata-controller", h.sync)
}

type metadataHandler struct {
	etcd           *ETCD
	nodeController controllerv1.NodeController
	ctx            context.Context
}

func (m *metadataHandler) sync(key string, node *v1.Node) (*v1.Node, error) {
	if node == nil {
		return nil, nil
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		logrus.Debug("waiting for node to be assigned for etcd controller")
		m.nodeController.EnqueueAfter(key, 5*time.Second)
		return node, nil
	}

	if key == nodeName {
		return m.handleSelf(node)
	}

	return node, nil
}

func (m *metadataHandler) handleSelf(node *v1.Node) (*v1.Node, error) {
	if node.Annotations[NodeNameAnnotation] == m.etcd.name &&
		node.Annotations[NodeAddressAnnotation] == m.etcd.address &&
		node.Labels[EtcdRoleLabel] == "true" &&
		node.Labels[ControlPlaneLabel] == "true" ||
		m.etcd.config.DisableETCD {
		return node, nil
	}

	node = node.DeepCopy()
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}

	node.Annotations[NodeNameAnnotation] = m.etcd.name
	node.Annotations[NodeAddressAnnotation] = m.etcd.address
	node.Labels[EtcdRoleLabel] = "true"
	node.Labels[MasterLabel] = "true"
	node.Labels[ControlPlaneLabel] = "true"

	return m.nodeController.Update(node)
}
