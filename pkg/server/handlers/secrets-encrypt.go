package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	"k8s.io/client-go/tools/pager"
	"k8s.io/utils/ptr"
)

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

func EncryptionStatus(control *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		status, err := encryptionStatus(control)
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

func encryptionStatus(control *config.Control) (EncryptionState, error) {
	state := EncryptionState{}
	if control.Runtime.Core == nil {
		return state, util.ErrCoreNotReady
	}

	providers, err := secretsencrypt.GetEncryptionProviders(control.Runtime)
	if os.IsNotExist(err) {
		return state, nil
	} else if err != nil {
		return state, err
	}
	if providers[len(providers)-1].Identity != nil && (providers[0].AESCBC != nil || providers[0].Secretbox != nil) {
		state.Enable = ptr.To(true)
	} else if control.EncryptSecrets && providers[0].Identity != nil && len(providers) == 1 {
		state.Enable = ptr.To(false)
	} else if !control.EncryptSecrets || providers[0].Identity != nil && (providers[1].AESCBC != nil || providers[1].Secretbox != nil) {
		state.Enable = ptr.To(false)
	}

	if err := verifyEncryptionHashAnnotation(control.Runtime, control.Runtime.Core.Core(), ""); err != nil {
		state.HashMatch = false
		state.HashError = err.Error()
	} else {
		state.HashMatch = true
	}
	stage, _, err := getEncryptionHashAnnotation(control.Runtime.Core.Core())
	if err != nil {
		return state, err
	}
	state.Stage = stage
	active := true
	for _, p := range providers {
		if p.AESCBC != nil {
			for _, aesKey := range p.AESCBC.Keys {
				typName := "AES-CBC " + aesKey.Name
				if active {
					active = false
					state.ActiveKey = typName
				} else {
					state.InactiveKeys = append(state.InactiveKeys, typName)
				}
			}
		}
		if p.Secretbox != nil {
			for _, sbKey := range p.Secretbox.Keys {
				typName := "XSalsa20-POLY1305 " + sbKey.Name
				if active {
					active = false
					state.ActiveKey = typName
				} else {
					state.InactiveKeys = append(state.InactiveKeys, typName)
				}
			}
		}
		if p.Identity != nil {
			active = false
		}
	}

	return state, nil
}

func encryptionEnable(ctx context.Context, control *config.Control, enable bool) error {
	providers, err := secretsencrypt.GetEncryptionProviders(control.Runtime)
	// Enable secrets encryption with an identity provider on a cluster that does not have any encryption config
	if err != nil && os.IsNotExist(err) && enable {
		if err := secretsencrypt.WriteIdentityConfig(control); err != nil {
			return err
		}
		return cluster.Save(ctx, control, true)
	} else if err != nil {
		return err
	}
	if len(providers) > 3 {
		return fmt.Errorf("more than 3 providers (%d) found in secrets encryption", len(providers))
	}
	curKeys, err := secretsencrypt.GetEncryptionKeys(control.Runtime)
	if err != nil {
		return err
	}

	if providers[len(providers)-1].Identity != nil && (providers[0].AESCBC != nil || providers[0].Secretbox != nil) && !enable {
		logrus.Infoln("Disabling secrets encryption")
		if err := secretsencrypt.WriteEncryptionConfig(control.Runtime, curKeys, control.EncryptProvider, enable); err != nil {
			return err
		}
	} else if !enable {
		logrus.Infoln("Secrets encryption already disabled")
		return nil
	} else if providers[0].Identity != nil && (providers[1].AESCBC != nil || providers[1].Secretbox != nil) && enable {
		foundKey := false
		// Check the rest of the providers (generally 2nd and 3rd) for the key type we are trying to enable.
		// If we find one, we can proceed.
		for _, p := range providers[1:] {
			if (control.EncryptProvider == secretsencrypt.AESCBCProvider && p.AESCBC != nil) ||
				(control.EncryptProvider == secretsencrypt.SecretBoxProvider && p.Secretbox != nil) {
				foundKey = true
			}
		}
		if !foundKey {
			return fmt.Errorf("cannot enable secrets encryption with %s key type, no keys found", control.EncryptProvider)
		}
		logrus.Infoln("Enabling secrets encryption")
		if err := secretsencrypt.WriteEncryptionConfig(control.Runtime, curKeys, control.EncryptProvider, enable); err != nil {
			return err
		}
	} else if enable {
		logrus.Infoln("Secrets encryption already enabled")
		return nil
	} else {
		return fmt.Errorf("unable to enable/disable secrets encryption, unknown configuration")
	}
	if err := cluster.Save(ctx, control, true); err != nil {
		return err
	}
	return reencryptAndRemoveKey(ctx, control, true, os.Getenv("NODE_NAME"))
}

func EncryptionConfig(ctx context.Context, control *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPut {
			util.SendError(fmt.Errorf("method not allowed"), resp, req, http.StatusMethodNotAllowed)
			return
		}

		if control.Runtime.Core == nil {
			util.SendError(util.ErrCoreNotReady, resp, req, http.StatusServiceUnavailable)
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
				err = encryptionPrepare(ctx, control, encryptReq.Force)
			case secretsencrypt.EncryptionRotate:
				err = encryptionRotate(ctx, control, encryptReq.Force)
			case secretsencrypt.EncryptionRotateKeys:
				err = encryptionRotateKeys(ctx, control)
			case secretsencrypt.EncryptionReencryptActive:
				err = encryptionReencrypt(ctx, control, encryptReq.Force, encryptReq.Skip)
			default:
				err = fmt.Errorf("unknown stage %s requested", *encryptReq.Stage)
			}
		} else if encryptReq.Enable != nil {
			err = encryptionEnable(ctx, control, *encryptReq.Enable)
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

func encryptionPrepare(ctx context.Context, control *config.Control, force bool) error {
	states := secretsencrypt.EncryptionStart + "-" + secretsencrypt.EncryptionReencryptFinished
	if err := verifyEncryptionHashAnnotation(control.Runtime, control.Runtime.Core.Core(), states); err != nil && !force {
		return err
	}
	if control.EncryptProvider == secretsencrypt.SecretBoxProvider {
		return fmt.Errorf("prepare does not support secretbox key type, use rotate-keys instead")
	}

	curKeys, err := secretsencrypt.GetEncryptionKeys(control.Runtime)
	if err != nil {
		return err
	}
	if err := AppendNewEncryptionKey(curKeys, control.EncryptProvider); err != nil {
		return err
	}

	if err := secretsencrypt.WriteEncryptionConfig(control.Runtime, curKeys, control.EncryptProvider, true); err != nil {
		return err
	}

	nodeName := os.Getenv("NODE_NAME")
	if err := secretsencrypt.WriteEncryptionHashAnnotation(ctx, control.Runtime, nodeName, false, secretsencrypt.EncryptionPrepare); err != nil {
		return err
	}

	return cluster.Save(ctx, control, true)
}

func encryptionRotate(ctx context.Context, control *config.Control, force bool) error {
	if err := verifyEncryptionHashAnnotation(control.Runtime, control.Runtime.Core.Core(), secretsencrypt.EncryptionPrepare); err != nil && !force {
		return err
	}
	if control.EncryptProvider == secretsencrypt.SecretBoxProvider {
		return fmt.Errorf("rotate does not support secretbox key type, use rotate-keys instead")
	}

	curKeys, err := secretsencrypt.GetEncryptionKeys(control.Runtime)
	if err != nil {
		return err
	}

	// Right rotate selected keys
	switch control.EncryptProvider {
	case secretsencrypt.AESCBCProvider:
		rotatedKeys := append(curKeys.AESCBCKeys[len(curKeys.AESCBCKeys)-1:], curKeys.AESCBCKeys[:len(curKeys.AESCBCKeys)-1]...)
		curKeys.AESCBCKeys = rotatedKeys
	case secretsencrypt.SecretBoxProvider:
		rotatedKeys := append(curKeys.SBKeys[len(curKeys.SBKeys)-1:], curKeys.SBKeys[:len(curKeys.SBKeys)-1]...)
		curKeys.SBKeys = rotatedKeys
	}

	if err := secretsencrypt.WriteEncryptionConfig(control.Runtime, curKeys, control.EncryptProvider, true); err != nil {
		return err
	}
	logrus.Infof("Encryption %s keys right rotated\n", control.EncryptProvider)

	nodeName := os.Getenv("NODE_NAME")
	if err := secretsencrypt.WriteEncryptionHashAnnotation(ctx, control.Runtime, nodeName, false, secretsencrypt.EncryptionRotate); err != nil {
		return err
	}

	return cluster.Save(ctx, control, true)
}

func encryptionReencrypt(ctx context.Context, control *config.Control, force bool, skip bool) error {
	if err := verifyEncryptionHashAnnotation(control.Runtime, control.Runtime.Core.Core(), secretsencrypt.EncryptionRotate); err != nil && !force {
		return err
	}
	if control.EncryptProvider == secretsencrypt.SecretBoxProvider {
		return fmt.Errorf("reencrypt does not support secretbox key type, use rotate-keys instead")
	}

	// Set the reencrypt-active annotation so other nodes know we are in the process of reencrypting.
	// As this stage is not persisted, we do not write the annotation to file
	nodeName := os.Getenv("NODE_NAME")
	if err := secretsencrypt.WriteEncryptionHashAnnotation(ctx, control.Runtime, nodeName, true, secretsencrypt.EncryptionReencryptActive); err != nil {
		return err
	}

	// We use a timeout of 10s for the reencrypt call, so finish the process as a go routine and return immediately.
	// No errors are returned to the user via CLI, any errors will be logged on the server
	go reencryptAndRemoveKey(ctx, control, skip, nodeName)
	return nil
}

func addAndRotateKeys(control *config.Control, keyType string) error {
	curKeys, err := secretsencrypt.GetEncryptionKeys(control.Runtime)
	if err != nil {
		return err
	}

	if err := AppendNewEncryptionKey(curKeys, keyType); err != nil {
		return err
	}

	if err := secretsencrypt.WriteEncryptionConfig(control.Runtime, curKeys, keyType, true); err != nil {
		return err
	}

	// Right rotate keyType keys
	if keyType == secretsencrypt.AESCBCProvider {
		rotatedKeys := append(curKeys.AESCBCKeys[len(curKeys.AESCBCKeys)-1:], curKeys.AESCBCKeys[:len(curKeys.AESCBCKeys)-1]...)
		curKeys.AESCBCKeys = rotatedKeys
	} else if keyType == secretsencrypt.SecretBoxProvider {
		rotatedKeys := append(curKeys.SBKeys[len(curKeys.SBKeys)-1:], curKeys.SBKeys[:len(curKeys.SBKeys)-1]...)
		curKeys.SBKeys = rotatedKeys
	}
	logrus.Infof("Rotating secrets-encryption %s keys\n", keyType)
	return secretsencrypt.WriteEncryptionConfig(control.Runtime, curKeys, keyType, true)
}

// encryptionRotateKeys is both adds and rotates keys, and sets the annotaiton that triggers the
// reencryption process. It is the preferred way to rotate keys, starting with v1.28
func encryptionRotateKeys(ctx context.Context, control *config.Control) error {
	states := secretsencrypt.EncryptionStart + "-" + secretsencrypt.EncryptionReencryptFinished
	if err := verifyEncryptionHashAnnotation(control.Runtime, control.Runtime.Core.Core(), states); err != nil {
		return err
	}

	if err := verifyRotateKeysSupport(control.Runtime.Core.Core()); err != nil {
		return err
	}

	reloadTime, reloadSuccesses, err := secretsencrypt.GetEncryptionConfigMetrics(control.Runtime, true)
	if err != nil {
		return err
	}

	// Set the reencrypt-active annotation so other nodes know we are in the process of reencrypting.
	// As this stage is not persisted, we do not write the annotation to file
	nodeName := os.Getenv("NODE_NAME")
	if err := secretsencrypt.WriteEncryptionHashAnnotation(ctx, control.Runtime, nodeName, true, secretsencrypt.EncryptionReencryptActive); err != nil {
		return err
	}

	if err := addAndRotateKeys(control, control.EncryptProvider); err != nil {
		return err
	}

	if err := secretsencrypt.WaitForEncryptionConfigReload(control.Runtime, reloadSuccesses, reloadTime); err != nil {
		return err
	}

	return reencryptAndRemoveKey(ctx, control, false, nodeName)
}

func reencryptAndRemoveKey(ctx context.Context, control *config.Control, skip bool, nodeName string) error {
	if err := updateSecrets(ctx, control, nodeName); err != nil {
		return err
	}

	// If skipping, revert back to the previous stage and do not remove the key
	if skip {
		return secretsencrypt.BootstrapEncryptionHashAnnotation(ctx, control.Runtime, nodeName)
	}

	// Remove old key. If there is only one of that key type, the cluster just
	// migrated between key types. Check for the other key type and remove that.
	// If that key type type doesn't exist, we are switching from the identity provider, so no key is removed.
	curKeys, err := secretsencrypt.GetEncryptionKeys(control.Runtime)
	if err != nil {
		return err
	}

	switch control.EncryptProvider {
	case secretsencrypt.AESCBCProvider:
		if len(curKeys.AESCBCKeys) == 1 && len(curKeys.SBKeys) > 0 {
			logrus.Infoln("Removing secretbox key: ", curKeys.SBKeys[len(curKeys.SBKeys)-1])
			curKeys.SBKeys = curKeys.SBKeys[:len(curKeys.SBKeys)-1]
		} else if len(curKeys.AESCBCKeys) == 1 && curKeys.Identity {
			logrus.Infoln("No keys to remove, switched from identity provider")
		} else {
			logrus.Infoln("Removing aescbc key: ", curKeys.AESCBCKeys[len(curKeys.AESCBCKeys)-1])
			curKeys.AESCBCKeys = curKeys.AESCBCKeys[:len(curKeys.AESCBCKeys)-1]
		}
	case secretsencrypt.SecretBoxProvider:
		if len(curKeys.SBKeys) == 1 && len(curKeys.AESCBCKeys) > 0 {
			logrus.Infoln("Removing aescbc key: ", curKeys.AESCBCKeys[len(curKeys.AESCBCKeys)-1])
			curKeys.AESCBCKeys = curKeys.AESCBCKeys[:len(curKeys.AESCBCKeys)-1]
		} else if len(curKeys.SBKeys) == 1 && curKeys.Identity {
			logrus.Infoln("No keys to remove, switched from identity provider")
		} else {
			logrus.Infoln("Removing secretbox key: ", curKeys.SBKeys[len(curKeys.SBKeys)-1])
			curKeys.SBKeys = curKeys.SBKeys[:len(curKeys.SBKeys)-1]
		}
	}

	if err := secretsencrypt.WriteEncryptionConfig(control.Runtime, curKeys, control.EncryptProvider, true); err != nil {
		return err
	}

	if err := secretsencrypt.WriteEncryptionHashAnnotation(ctx, control.Runtime, nodeName, false, secretsencrypt.EncryptionReencryptFinished); err != nil {
		return err
	}

	return cluster.Save(ctx, control, true)
}

func updateSecrets(ctx context.Context, control *config.Control, nodeName string) error {
	k8s := control.Runtime.K8s
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

func AppendNewEncryptionKey(keys *secretsencrypt.EncryptionKeys, keyType string) error {
	var keyPrefix string
	switch keyType {
	case secretsencrypt.AESCBCProvider:
		keyPrefix = "aescbckey-"
	case secretsencrypt.SecretBoxProvider:
		keyPrefix = "secretboxkey-"
	}

	keyByte := make([]byte, secretsencrypt.KeySize)
	if _, err := rand.Read(keyByte); err != nil {
		return err
	}
	encodedKey := base64.StdEncoding.EncodeToString(keyByte)

	newKey := []apiserverconfigv1.Key{
		{
			Name:   keyPrefix + time.Now().Format(time.RFC3339),
			Secret: encodedKey,
		},
	}
	if keyType == secretsencrypt.AESCBCProvider {
		keys.AESCBCKeys = append(keys.AESCBCKeys, newKey...)
	} else if keyType == secretsencrypt.SecretBoxProvider {
		keys.SBKeys = append(keys.SBKeys, newKey...)
	}
	logrus.Infoln("Adding secrets-encryption key: ", newKey)
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
