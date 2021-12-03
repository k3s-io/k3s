package secretsencrypt

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/k3s/pkg/cluster"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/generated/clientset/versioned/scheme"
	coreclient "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	coregetter "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	controllerAgentName        string = "reencrypt-controller"
	secretsUpdateStartEvent    string = "SecretsUpdateStart"
	secretsProgressEvent       string = "SecretsProgress"
	secretsUpdateCompleteEvent string = "SecretsUpdateComplete"
	secretsUpdateErrorEvent    string = "SecretsUpdateError"
)

type handler struct {
	ctx           context.Context
	controlConfig *config.Control
	nodes         coreclient.NodeController
	secrets       coreclient.SecretController
	recorder      record.EventRecorder
}

func Register(
	ctx context.Context,
	kubernetes kubernetes.Interface,
	controlConfig *config.Control,
	nodes coreclient.NodeController,
	secrets coreclient.SecretController,
	events coregetter.EventInterface,
) error {
	h := &handler{
		ctx:           ctx,
		controlConfig: controlConfig,
		nodes:         nodes,
		secrets:       secrets,
		recorder:      buildEventRecorder(events),
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

// onChangeNode handles changes to Nodes. We are looking for a specific annotation change
func (h *handler) onChangeNode(key string, node *corev1.Node) (*corev1.Node, error) {
	if node == nil {
		return nil, nil
	}

	ann, ok := node.Annotations[EncryptionHashAnnotation]
	if !ok {
		return node, nil
	}
	split := strings.Split(ann, "-")
	if len(split) != 2 {
		return node, fmt.Errorf("invalid annotation %s found on node %s", ann, node.ObjectMeta.Name)
	}
	stage := split[0]
	hash := split[1]

	// Validate the specific stage and the request via sha256 hash
	if stage != EncryptionReencryptRequest {
		return node, nil
	}
	curHash, err := GenEncryptionConfigHash(h.controlConfig.Runtime)
	if err != nil {
		return node, err
	} else if curHash != hash {
		return node, fmt.Errorf("invalid hash: %s found on node %s", hash, node.ObjectMeta.Name)
	}

	if err := h.verifyReencryptStage(curHash); err != nil {
		return node, err
	}

	if err := WriteEncryptionHashAnnotation(h.controlConfig.Runtime, node, EncryptionReencryptActive, true); err != nil {
		return node, err
	}

	if err := h.updateSecrets(node); err != nil {
		return node, err
	}

	if h.controlConfig.EncryptSkip {
		return node, nil
	}

	// Remove last key
	curKeys, err := GetEncryptionKeys(h.controlConfig.Runtime)
	if err != nil {
		return node, err
	}

	curKeys = curKeys[:len(curKeys)-1]
	if err = WriteEncryptionConfig(h.controlConfig.Runtime, curKeys, true); err != nil {
		return node, err
	}
	logrus.Infoln("Removed key: ", curKeys[len(curKeys)-1])
	if err := WriteEncryptionHashAnnotation(h.controlConfig.Runtime, node, EncryptionReencryptFinished, false); err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}
	if err := cluster.Save(h.ctx, h.controlConfig, h.controlConfig.Runtime.EtcdConfig, true); err != nil {
		return node, err
	}
	return node, nil
}

// verifyReencryptStage ensure that there is only one active reencryption at a time
func (h *handler) verifyReencryptStage(curHash string) error {
	nodes, err := h.nodes.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, node := range nodes.Items {
		if ann, ok := node.Annotations[EncryptionHashAnnotation]; ok {
			split := strings.Split(ann, "-")
			if len(split) != 2 {
				return fmt.Errorf("invalid annotation %s found on node %s", ann, node.ObjectMeta.Name)
			}
			stage := split[0]
			hash := split[1]
			if stage == EncryptionReencryptActive && hash == curHash {
				return fmt.Errorf("another reencrypt is already active")
			}
		}
	}
	return nil
}

func (h *handler) updateSecrets(node *corev1.Node) error {
	secretList, err := h.secrets.List("", metav1.ListOptions{})
	if err != nil {
		return err
	}
	numOfSecrets := len(secretList.Items)
	h.recorder.Event(node, corev1.EventTypeNormal, secretsUpdateStartEvent, "started reencrypting secrets")

	for i, s := range secretList.Items {
		if _, err := h.secrets.Update(&s); err != nil {
			return fmt.Errorf("failed to reencrypted secret: %v", err)
		}
		if i != 0 && i%10 == 0 {
			h.recorder.Eventf(node, corev1.EventTypeNormal, secretsProgressEvent, "reencrypted %d of %d secrets", i, numOfSecrets)
		}
	}
	h.recorder.Eventf(node, corev1.EventTypeNormal, secretsUpdateCompleteEvent, "completed update of %d secrets", numOfSecrets)
	return nil
}
