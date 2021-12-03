package reencrypt

import (
	"context"

	"github.com/rancher/k3s/pkg/generated/clientset/versioned/scheme"
	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/wrangler/pkg/apply"
	coreclient "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	coregetter "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName = "reencrypt-controller"
)

var (
	encryptionHashAnnotation = version.Program + ".io/encryption-config-hash"
)

type handler struct {
	nodeCache coreclient.NodeCache
	recorder  record.EventRecorder
}

func Register(
	ctx context.Context,
	kubernetes kubernetes.Interface,
	apply apply.Apply,
	nodes coreclient.NodeController,
	events coregetter.EventInterface,
) error {
	h := &handler{
		nodeCache: nodes.Cache(),
		recorder:  buildEventRecorder(events),
	}

	nodes.OnChange(ctx, "reencrypt-controller", h.onChangeNode)
	return nil
}

func buildEventRecorder(events coregetter.EventInterface) record.EventRecorder {
	// Create event broadcaster
	logrus.Info("Creating reencrypt event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)
	eventBroadcaster.StartRecordingToSink(&coregetter.EventSinkImpl{Interface: events})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
}

// onChangeNode handles changes to Nodes. We need to handle this as we may need to kick the DaemonSet
// to add or remove pods from nodes if labels have changed.
func (h *handler) onChangeNode(key string, node *corev1.Node) (*corev1.Node, error) {
	if node == nil {
		return nil, nil
	}
	h.recorder.Event(node, corev1.EventTypeWarning, "ErrHelloWorld", "this is a sample event")

	if _, ok := node.Annotations[encryptionHashAnnotation]; !ok {
		return node, nil
	}

	return node, nil
}
