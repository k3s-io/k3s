package secretsencrypt

import (
	"context"
	"fmt"
	"strings"

	"github.com/rancher/k3s/pkg/cluster"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/util"
	coreclient "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/pager"
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
	k8s kubernetes.Interface,
	controlConfig *config.Control,
	nodes coreclient.NodeController,
	secrets coreclient.SecretController,
) error {
	h := &handler{
		ctx:           ctx,
		controlConfig: controlConfig,
		nodes:         nodes,
		secrets:       secrets,
		recorder:      util.BuildControllerEventRecorder(k8s, controllerAgentName),
	}

	nodes.OnChange(ctx, "reencrypt-controller", h.onChangeNode)
	return nil
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

	if valid, err := h.validateReencryptStage(node, ann); err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	} else if !valid {
		return node, nil
	}

	reencryptHash, err := GenReencryptHash(h.controlConfig.Runtime, EncryptionReencryptActive)
	if err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}
	ann = EncryptionReencryptActive + "-" + reencryptHash
	node.Annotations[EncryptionHashAnnotation] = ann
	node, err = h.nodes.Update(node)
	if err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}

	if err := h.updateSecrets(node); err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}

	// If skipping, revert back to the previous stage
	if h.controlConfig.EncryptSkip {
		BootstrapEncryptionHashAnnotation(node, h.controlConfig.Runtime)
		if node, err := h.nodes.Update(node); err != nil {
			return node, err
		}
		return node, nil
	}

	// Remove last key
	curKeys, err := GetEncryptionKeys(h.controlConfig.Runtime)
	if err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}

	curKeys = curKeys[:len(curKeys)-1]
	if err = WriteEncryptionConfig(h.controlConfig.Runtime, curKeys, true); err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}
	logrus.Infoln("Removed key: ", curKeys[len(curKeys)-1])
	if err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}
	if err := WriteEncryptionHashAnnotation(h.controlConfig.Runtime, node, EncryptionReencryptFinished); err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}
	if err := cluster.Save(h.ctx, h.controlConfig, h.controlConfig.Runtime.EtcdConfig, true); err != nil {
		h.recorder.Event(node, corev1.EventTypeWarning, secretsUpdateErrorEvent, err.Error())
		return node, err
	}
	return node, nil
}

// validateReencryptStage ensures that the request for reencryption is valid and
// that there is only one active reencryption at a time
func (h *handler) validateReencryptStage(node *corev1.Node, annotation string) (bool, error) {

	split := strings.Split(annotation, "-")
	if len(split) != 2 {
		err := fmt.Errorf("invalid annotation %s found on node %s", annotation, node.ObjectMeta.Name)
		return false, err
	}
	stage := split[0]
	hash := split[1]

	// Validate the specific stage and the request via sha256 hash
	if stage != EncryptionReencryptRequest {
		return false, nil
	}
	if reencryptRequestHash, err := GenReencryptHash(h.controlConfig.Runtime, EncryptionReencryptRequest); err != nil {
		return false, err
	} else if reencryptRequestHash != hash {
		err = fmt.Errorf("invalid hash: %s found on node %s", hash, node.ObjectMeta.Name)
		return false, err
	}

	nodes, err := h.nodes.List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	reencryptActiveHash, err := GenReencryptHash(h.controlConfig.Runtime, EncryptionReencryptActive)
	if err != nil {
		return false, err
	}
	for _, node := range nodes.Items {
		if ann, ok := node.Annotations[EncryptionHashAnnotation]; ok {
			split := strings.Split(ann, "-")
			if len(split) != 2 {
				return false, fmt.Errorf("invalid annotation %s found on node %s", ann, node.ObjectMeta.Name)
			}
			stage := split[0]
			hash := split[1]
			if stage == EncryptionReencryptActive && hash == reencryptActiveHash {
				return false, fmt.Errorf("another reencrypt is already active")
			}
		}
	}
	return true, nil
}

func (h *handler) updateSecrets(node *corev1.Node) error {
	secretPager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		return h.secrets.List("", opts)
	}))
	i := 0
	secretPager.EachListItem(h.ctx, metav1.ListOptions{}, func(obj runtime.Object) error {
		if secret, ok := obj.(*corev1.Secret); ok {
			if _, err := h.secrets.Update(secret); err != nil {
				return fmt.Errorf("failed to reencrypted secret: %v", err)
			}
			if i != 0 && i%10 == 0 {
				h.recorder.Eventf(node, corev1.EventTypeNormal, secretsProgressEvent, "reencrypted %d secrets", i)
			}
			i++
		}
		return nil
	})
	h.recorder.Eventf(node, corev1.EventTypeNormal, secretsUpdateCompleteEvent, "completed reencrypt of %d secrets", i)
	return nil
}
