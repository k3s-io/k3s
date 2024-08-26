package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/k3s-io/k3s/pkg/cluster"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/secretsencrypt"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/pager"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
)

const aescbcKeySize = 32

type EncryptionState struct {
	Stage        string   `json:"stage"`
	ActiveKey    string   `json:"activekey"`
	Enable       *bool    `json:"enable,omitempty"`
	HashMatch    bool     `json:"hashmatch,omitempty"`
	HashError    string   `json:"hasherror,omitempty"`
	InactiveKeys []string `json:"inactivekeys,omitempty"`
}

type EncryptionRequest struct {
	Stage  *string `json:"stage,omitempty"`
	Enable *bool   `json:"enable,omitempty"`
	Force  bool    `json:"force"`
	Skip   bool    `json:"skip"`
}

func getEncryptionRequest(req *http.Request) (*EncryptionRequest, error) {
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	result := &EncryptionRequest{}
	err = json.Unmarshal(b, &result)
	return result, err
}

func encryptionStatusHandler(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		status, err := encryptionStatus(server)
		if err != nil {
			util.SendErrorWithID(err, "secret-encrypt", resp, req, http.StatusInternalServerError)
			return
		}
		b, err := json.Marshal(status)
		if err != nil {
			util.SendErrorWithID(err, "secret-encrypt", resp, req, http.StatusInternalServerError)
			return
		}
		resp.Header().Set("Content-Type", "application/json")
		resp.Write(b)
	})
}

func encryptionStatus(server *config.Control) (EncryptionState, error) {
	state := EncryptionState{}
	providers, err := secretsencrypt.GetEncryptionProviders(server.Runtime)
	if os.IsNotExist(err) {
		return state, nil
	} else if err != nil {
		return state, err
	}
	if providers[1].Identity != nil && providers[0].AESCBC != nil {
		state.Enable = ptr.To(true)
	} else if providers[0].Identity != nil && providers[1].AESCBC != nil || !server.EncryptSecrets {
		state.Enable = ptr.To(false)
	}

	if err := verifyEncryptionHashAnnotation(server.Runtime, server.Runtime.Core.Core(), ""); err != nil {
		state.HashMatch = false
		state.HashError = err.Error()
	} else {
		state.HashMatch = true
	}
	stage, _, err := getEncryptionHashAnnotation(server.Runtime.Core.Core())
	if err != nil {
		return state, err
	}
	state.Stage = stage
	active := true
	for _, p := range providers {
		if p.AESCBC != nil {
			for _, aesKey := range p.AESCBC.Keys {
				if active {
					active = false
					state.ActiveKey = aesKey.Name
				} else {
					state.InactiveKeys = append(state.InactiveKeys, aesKey.Name)
				}
			}
		}
		if p.Identity != nil {
			active = false
		}
	}

	return state, nil
}

func encryptionEnable(ctx context.Context, server *config.Control, enable bool) error {
	providers, err := secretsencrypt.GetEncryptionProviders(server.Runtime)
	if err != nil {
		return err
	}
	if len(providers) > 2 {
		return fmt.Errorf("more than 2 providers (%d) found in secrets encryption", len(providers))
	}
	curKeys, err := secretsencrypt.GetEncryptionKeys(server.Runtime, false)
	if err != nil {
		return err
	}
	if providers[1].Identity != nil && providers[0].AESCBC != nil && !enable {
		logrus.Infoln("Disabling secrets encryption")
		if err := secretsencrypt.WriteEncryptionConfig(server.Runtime, curKeys, enable); err != nil {
			return err
		}
	} else if !enable {
		logrus.Infoln("Secrets encryption already disabled")
		return nil
	} else if providers[0].Identity != nil && providers[1].AESCBC != nil && enable {
		logrus.Infoln("Enabling secrets encryption")
		if err := secretsencrypt.WriteEncryptionConfig(server.Runtime, curKeys, enable); err != nil {
			return err
		}
	} else if enable {
		logrus.Infoln("Secrets encryption already enabled")
		return nil
	} else {
		return fmt.Errorf("unable to enable/disable secrets encryption, unknown configuration")
	}
	if err := cluster.Save(ctx, server, true); err != nil {
		return err
	}
	return reencryptAndRemoveKey(ctx, server, true, os.Getenv("NODE_NAME"))
}

func encryptionConfigHandler(ctx context.Context, server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			util.SendError(fmt.Errorf("method not allowed"), resp, req, http.StatusMethodNotAllowed)
			return
		}
		encryptReq, err := getEncryptionRequest(req)
		if err != nil {
			util.SendError(err, resp, req, http.StatusBadRequest)
			return
		}
		if encryptReq.Stage != nil {
			switch *encryptReq.Stage {
			case secretsencrypt.EncryptionPrepare:
				err = encryptionPrepare(ctx, server, encryptReq.Force)
			case secretsencrypt.EncryptionRotate:
				err = encryptionRotate(ctx, server, encryptReq.Force)
			case secretsencrypt.EncryptionRotateKeys:
				err = encryptionRotateKeys(ctx, server)
			case secretsencrypt.EncryptionReencryptActive:
				err = encryptionReencrypt(ctx, server, encryptReq.Force, encryptReq.Skip)
			default:
				err = fmt.Errorf("unknown stage %s requested", *encryptReq.Stage)
			}
		} else if encryptReq.Enable != nil {
			err = encryptionEnable(ctx, server, *encryptReq.Enable)
		}

		if err != nil {
			util.SendErrorWithID(err, "secret-encrypt", resp, req, http.StatusBadRequest)
			return
		}
		// If a user kills the k3s server immediately after this call, we run into issues where the files
		// have not yet been written. This sleep ensures that things have time to sync to disk before
		// the request completes.
		time.Sleep(1 * time.Second)
		resp.WriteHeader(http.StatusOK)
	})
}

func encryptionPrepare(ctx context.Context, server *config.Control, force bool) error {
	states := secretsencrypt.EncryptionStart + "-" + secretsencrypt.EncryptionReencryptFinished
	if err := verifyEncryptionHashAnnotation(server.Runtime, server.Runtime.Core.Core(), states); err != nil && !force {
		return err
	}

	curKeys, err := secretsencrypt.GetEncryptionKeys(server.Runtime, false)
	if err != nil {
		return err
	}

	if err := AppendNewEncryptionKey(&curKeys); err != nil {
		return err
	}
	logrus.Infoln("Adding secrets-encryption key: ", curKeys[len(curKeys)-1])

	if err := secretsencrypt.WriteEncryptionConfig(server.Runtime, curKeys, true); err != nil {
		return err
	}
	nodeName := os.Getenv("NODE_NAME")
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := server.Runtime.Core.Core().V1().Node().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return secretsencrypt.WriteEncryptionHashAnnotation(server.Runtime, node, false, secretsencrypt.EncryptionPrepare)
	})
	if err != nil {
		return err
	}
	return cluster.Save(ctx, server, true)
}

func encryptionRotate(ctx context.Context, server *config.Control, force bool) error {
	if err := verifyEncryptionHashAnnotation(server.Runtime, server.Runtime.Core.Core(), secretsencrypt.EncryptionPrepare); err != nil && !force {
		return err
	}

	curKeys, err := secretsencrypt.GetEncryptionKeys(server.Runtime, false)
	if err != nil {
		return err
	}

	// Right rotate elements
	rotatedKeys := append(curKeys[len(curKeys)-1:], curKeys[:len(curKeys)-1]...)

	if err = secretsencrypt.WriteEncryptionConfig(server.Runtime, rotatedKeys, true); err != nil {
		return err
	}
	logrus.Infoln("Encryption keys right rotated")
	nodeName := os.Getenv("NODE_NAME")
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := server.Runtime.Core.Core().V1().Node().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return secretsencrypt.WriteEncryptionHashAnnotation(server.Runtime, node, false, secretsencrypt.EncryptionRotate)
	})
	if err != nil {
		return err
	}
	return cluster.Save(ctx, server, true)
}

func encryptionReencrypt(ctx context.Context, server *config.Control, force bool, skip bool) error {
	if err := verifyEncryptionHashAnnotation(server.Runtime, server.Runtime.Core.Core(), secretsencrypt.EncryptionRotate); err != nil && !force {
		return err
	}
	// Set the reencrypt-active annotation so other nodes know we are in the process of reencrypting.
	// As this stage is not persisted, we do not write the annotation to file
	nodeName := os.Getenv("NODE_NAME")
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := server.Runtime.Core.Core().V1().Node().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return secretsencrypt.WriteEncryptionHashAnnotation(server.Runtime, node, true, secretsencrypt.EncryptionReencryptActive)
	}); err != nil {
		return err
	}

	return reencryptAndRemoveKey(ctx, server, skip, nodeName)
}

func addAndRotateKeys(server *config.Control) error {
	curKeys, err := secretsencrypt.GetEncryptionKeys(server.Runtime, false)
	if err != nil {
		return err
	}

	if err := AppendNewEncryptionKey(&curKeys); err != nil {
		return err
	}
	logrus.Infoln("Adding secrets-encryption key: ", curKeys[len(curKeys)-1])

	if err := secretsencrypt.WriteEncryptionConfig(server.Runtime, curKeys, true); err != nil {
		return err
	}

	// Right rotate elements
	rotatedKeys := append(curKeys[len(curKeys)-1:], curKeys[:len(curKeys)-1]...)
	logrus.Infoln("Rotating secrets-encryption keys")
	return secretsencrypt.WriteEncryptionConfig(server.Runtime, rotatedKeys, true)
}

// encryptionRotateKeys is both adds and rotates keys, and sets the annotaiton that triggers the
// reencryption process. It is the preferred way to rotate keys, starting with v1.28
func encryptionRotateKeys(ctx context.Context, server *config.Control) error {
	states := secretsencrypt.EncryptionStart + "-" + secretsencrypt.EncryptionReencryptFinished
	if err := verifyEncryptionHashAnnotation(server.Runtime, server.Runtime.Core.Core(), states); err != nil {
		return err
	}

	if err := verifyRotateKeysSupport(server.Runtime.Core.Core()); err != nil {
		return err
	}

	reloadTime, reloadSuccesses, err := secretsencrypt.GetEncryptionConfigMetrics(server.Runtime, true)
	if err != nil {
		return err
	}

	// Set the reencrypt-active annotation so other nodes know we are in the process of reencrypting.
	// As this stage is not persisted, we do not write the annotation to file
	nodeName := os.Getenv("NODE_NAME")
	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := server.Runtime.Core.Core().V1().Node().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return secretsencrypt.WriteEncryptionHashAnnotation(server.Runtime, node, true, secretsencrypt.EncryptionReencryptActive)
	}); err != nil {
		return err
	}

	if err := addAndRotateKeys(server); err != nil {
		return err
	}

	if err := secretsencrypt.WaitForEncryptionConfigReload(server.Runtime, reloadSuccesses, reloadTime); err != nil {
		return err
	}

	return reencryptAndRemoveKey(ctx, server, false, nodeName)
}

func reencryptAndRemoveKey(ctx context.Context, server *config.Control, skip bool, nodeName string) error {
	if err := updateSecrets(ctx, server, nodeName); err != nil {
		return err
	}

	// If skipping, revert back to the previous stage and do not remove the key
	if skip {
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			node, err := server.Runtime.Core.Core().V1().Node().Get(nodeName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			secretsencrypt.BootstrapEncryptionHashAnnotation(node, server.Runtime)
			_, err = server.Runtime.Core.Core().V1().Node().Update(node)
			return err
		})
		return err
	}

	// Remove last key
	curKeys, err := secretsencrypt.GetEncryptionKeys(server.Runtime, false)
	if err != nil {
		return err
	}

	logrus.Infoln("Removing key: ", curKeys[len(curKeys)-1])
	curKeys = curKeys[:len(curKeys)-1]
	if err = secretsencrypt.WriteEncryptionConfig(server.Runtime, curKeys, true); err != nil {
		return err
	}

	if err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := server.Runtime.Core.Core().V1().Node().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return secretsencrypt.WriteEncryptionHashAnnotation(server.Runtime, node, false, secretsencrypt.EncryptionReencryptFinished)
	}); err != nil {
		return err
	}

	return cluster.Save(ctx, server, true)
}

func updateSecrets(ctx context.Context, server *config.Control, nodeName string) error {
	restConfig, err := clientcmd.BuildConfigFromFlags("", server.Runtime.KubeConfigSupervisor)
	if err != nil {
		return err
	}
	// For secrets we need a much higher QPS than default
	restConfig.QPS = secretsencrypt.SecretQPS
	restConfig.Burst = secretsencrypt.SecretBurst
	k8s, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	nodeRef := &corev1.ObjectReference{
		Kind:      "Node",
		Name:      nodeName,
		UID:       types.UID(nodeName),
		Namespace: "",
	}

	// For backwards compatibility with the old controller, we use an event recorder instead of logrus
	recorder := util.BuildControllerEventRecorder(k8s, "secrets-reencrypt", metav1.NamespaceDefault)

	secretPager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		return k8s.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, opts)
	}))
	secretPager.PageSize = secretsencrypt.SecretListPageSize

	i := 0
	if err := secretPager.EachListItem(ctx, metav1.ListOptions{}, func(obj runtime.Object) error {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return errors.New("failed to convert object to Secret")
		}
		if _, err := k8s.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil && !apierrors.IsConflict(err) {
			recorder.Eventf(nodeRef, corev1.EventTypeWarning, secretsencrypt.SecretsUpdateErrorEvent, "failed to update secret: %v", err)
			return fmt.Errorf("failed to update secret: %v", err)
		}
		if i != 0 && i%50 == 0 {
			recorder.Eventf(nodeRef, corev1.EventTypeNormal, secretsencrypt.SecretsProgressEvent, "reencrypted %d secrets", i)
		}
		i++
		return nil
	}); err != nil {
		return err
	}
	recorder.Eventf(nodeRef, corev1.EventTypeNormal, secretsencrypt.SecretsUpdateCompleteEvent, "reencrypted %d secrets", i)
	return nil
}

func AppendNewEncryptionKey(keys *[]apiserverconfigv1.Key) error {
	aescbcKey := make([]byte, aescbcKeySize)
	_, err := rand.Read(aescbcKey)
	if err != nil {
		return err
	}
	encodedKey := base64.StdEncoding.EncodeToString(aescbcKey)

	newKey := []apiserverconfigv1.Key{
		{
			Name:   "aescbckey-" + time.Now().Format(time.RFC3339),
			Secret: encodedKey,
		},
	}
	*keys = append(*keys, newKey...)
	return nil
}

func getEncryptionHashAnnotation(core core.Interface) (string, string, error) {
	nodeName := os.Getenv("NODE_NAME")
	node, err := core.V1().Node().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	if _, ok := node.Labels[util.ControlPlaneRoleLabelKey]; !ok {
		return "", "", fmt.Errorf("cannot manage secrets encryption on non control-plane node %s", nodeName)
	}
	if ann, ok := node.Annotations[secretsencrypt.EncryptionHashAnnotation]; ok {
		split := strings.Split(ann, "-")
		if len(split) != 2 {
			return "", "", fmt.Errorf("invalid annotation %s found on node %s", ann, nodeName)
		}
		return split[0], split[1], nil
	}
	return "", "", fmt.Errorf("missing annotation on node %s", nodeName)
}

// verifyRotateKeysSupport checks that the k3s version is at least v1.28.0 on all control-plane nodes
func verifyRotateKeysSupport(core core.Interface) error {
	labelSelector := labels.Set{util.ControlPlaneRoleLabelKey: "true"}.String()
	nodes, err := core.V1().Node().List(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return err
	}
	for _, node := range nodes.Items {
		kubver, err := semver.ParseTolerant(node.Status.NodeInfo.KubeletVersion)
		if err != nil {
			return fmt.Errorf("failed to parse kubelet version %s: %v", node.Status.NodeInfo.KubeletVersion, err)
		}
		supportVer, err := semver.Make("1.28.0")
		if err != nil {
			return err
		}
		if kubver.LT(supportVer) {
			return fmt.Errorf("node %s is running k3s version %s that does not support rotate-keys", node.ObjectMeta.Name, kubver.String())
		}
	}
	return nil
}

// verifyEncryptionHashAnnotation checks that all nodes are on the same stage,
// and that a request for new stage is valid
func verifyEncryptionHashAnnotation(runtime *config.ControlRuntime, core core.Interface, prevStage string) error {
	var firstHash string
	var firstNodeName string
	first := true
	labelSelector := labels.Set{util.ControlPlaneRoleLabelKey: "true"}.String()
	nodes, err := core.V1().Node().List(metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return err
	}
	for _, node := range nodes.Items {
		hash, ok := node.Annotations[secretsencrypt.EncryptionHashAnnotation]
		if ok && first {
			firstHash = hash
			first = false
			firstNodeName = node.ObjectMeta.Name
		} else if ok && hash != firstHash {
			return fmt.Errorf("hash does not match between %s and %s", firstNodeName, node.ObjectMeta.Name)
		}
	}

	if prevStage == "" {
		return nil
	}

	oldStage, oldHash, err := getEncryptionHashAnnotation(core)
	if err != nil {
		return err
	}

	encryptionConfigHash, err := secretsencrypt.GenEncryptionConfigHash(runtime)
	if err != nil {
		return err
	}
	if !strings.Contains(prevStage, oldStage) {
		return fmt.Errorf("incorrect stage: %s found on node %s", oldStage, nodes.Items[0].ObjectMeta.Name)
	} else if oldHash != encryptionConfigHash {
		return fmt.Errorf("invalid hash: %s found on node %s", oldHash, nodes.Items[0].ObjectMeta.Name)
	}

	return nil
}
