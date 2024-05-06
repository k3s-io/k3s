package etcd

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/util"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
	nodeutil "k8s.io/kubernetes/pkg/controller/util/node"
)

func registerMetadataHandlers(ctx context.Context, etcd *ETCD) {
	nodes := etcd.config.Runtime.Core.Core().V1().Node()
	h := &metadataHandler{
		etcd:           etcd,
		nodeController: nodes,
		ctx:            ctx,
		once:           &sync.Once{},
	}

	logrus.Infof("Starting managed etcd node metadata controller")
	nodes.OnChange(ctx, "managed-etcd-metadata-controller", h.sync)
}

type metadataHandler struct {
	etcd           *ETCD
	nodeController controllerv1.NodeController
	ctx            context.Context
	once           *sync.Once
}

func (m *metadataHandler) sync(key string, node *v1.Node) (*v1.Node, error) {

	if node == nil {
		return nil, nil
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		logrus.Debug("waiting for node name to be assigned for managed etcd node metadata controller")
		m.nodeController.EnqueueAfter(key, 5*time.Second)
		return node, nil
	}

	if key == nodeName {
		return m.handleSelf(node)
	}

	return node, nil
}

// checkReset ensures that member removal annotations are cleared when the cluster is reset.
// This is done here instead of in the member controller, as the member removal controller is
// not guaranteed to run on the node that was reset.
func (m *metadataHandler) checkReset() {
	if resetDone, _ := m.etcd.IsReset(); resetDone {
		labelSelector := labels.Set{util.ETCDRoleLabelKey: "true"}.String()
		nodes, err := m.nodeController.List(metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			logrus.Errorf("Failed to list etcd nodes: %v", err)
			return
		}
		for _, n := range nodes.Items {
			node := &n
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				_, remove := node.Annotations[removalAnnotation]
				_, removed := node.Annotations[removedNodeNameAnnotation]
				if remove || removed {
					node = node.DeepCopy()
					delete(node.Annotations, removalAnnotation)
					delete(node.Annotations, removedNodeNameAnnotation)
					node, err = m.nodeController.Update(node)
					return err
				}
				return nil
			})
			if err != nil {
				logrus.Errorf("Failed to clear removal annotations from node %s after cluster reset: %v", node.Name, err)
			} else {
				logrus.Infof("Cleared etcd member removal annotations from node %s after cluster reset", node.Name)
			}
		}
		if err := m.etcd.clearReset(); err != nil {
			logrus.Errorf("Failed to delete etcd cluster-reset file: %v", err)
		}
	}
}

func (m *metadataHandler) handleSelf(node *v1.Node) (*v1.Node, error) {
	if m.etcd.config.DisableETCD {
		if node.Annotations[NodeNameAnnotation] == "" &&
			node.Annotations[NodeAddressAnnotation] == "" &&
			node.Labels[util.ETCDRoleLabelKey] == "" {
			return node, nil
		}

		node = node.DeepCopy()
		if node.Annotations == nil {
			node.Annotations = map[string]string{}
		}
		if node.Labels == nil {
			node.Labels = map[string]string{}
		}

		if find, _ := nodeutil.GetNodeCondition(&node.Status, etcdStatusType); find >= 0 {
			node.Status.Conditions = append(node.Status.Conditions[:find], node.Status.Conditions[find+1:]...)
		}

		delete(node.Annotations, NodeNameAnnotation)
		delete(node.Annotations, NodeAddressAnnotation)
		delete(node.Labels, util.ETCDRoleLabelKey)

		return m.nodeController.Update(node)
	}

	m.once.Do(m.checkReset)

	if node.Annotations[NodeNameAnnotation] == m.etcd.name &&
		node.Annotations[NodeAddressAnnotation] == m.etcd.address &&
		node.Labels[util.ETCDRoleLabelKey] == "true" {
		return node, nil
	}

	node = node.DeepCopy()
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}

	node.Annotations[NodeNameAnnotation] = m.etcd.name
	node.Annotations[NodeAddressAnnotation] = m.etcd.address
	node.Labels[util.ETCDRoleLabelKey] = "true"

	return m.nodeController.Update(node)
}
