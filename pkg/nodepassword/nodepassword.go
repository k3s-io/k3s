package nodepassword

import (
	"context"
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/pkg/authenticator/hash"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/errors"
	"github.com/k3s-io/k3s/pkg/version"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	// Hasher provides the algorithm for generating and verifying hashes
	Hasher          = hash.NewSCrypt()
	ErrVerifyFailed = errVerifyFailed()

	SecretTypeNodePassword = v1.SecretType(version.Program + ".cattle.io/node-password")
)

type passwordError struct {
	node string
	err  error
}

func (e *passwordError) Error() string {
	return fmt.Sprintf("unable to verify password for node %s: %v", e.node, e.err)
}

func (e *passwordError) Is(target error) bool {
	return target == ErrVerifyFailed
}

func (e *passwordError) Unwrap() error {
	return e.err
}

func errVerifyFailed() error { return &passwordError{} }

func getSecretName(nodeName string) string {
	return strings.ToLower(nodeName + ".node-password." + version.Program)
}

func (npc *nodePasswordController) verifyHash(nodeName, pass string, cached bool) error {
	secret, err := npc.getSecret(nodeName, cached)
	if err != nil {
		return &passwordError{node: nodeName, err: err}
	}
	if hash, ok := secret.Data["hash"]; ok {
		if err := Hasher.VerifyHash(string(hash), pass); err != nil {
			return &passwordError{node: nodeName, err: err}
		}
		return nil
	}
	return &passwordError{node: nodeName, err: errors.New("password hash not found in node secret")}
}

// ensure verifies a node-password secret if it exists, otherwise creates one.
func (npc *nodePasswordController) ensure(nodeName, pass string) error {
	// Try cache, then apiserver, before create (avoid AlreadyExists on cache lag).
	if err := npc.verifyHash(nodeName, pass, true); err == nil {
		return nil
	} else if !secretNotFound(err) {
		return err
	}
	if err := npc.verifyHash(nodeName, pass, false); err == nil {
		return nil
	} else if !secretNotFound(err) {
		return err
	}

	hash, err := Hasher.CreateHash(pass)
	if err != nil {
		return &passwordError{node: nodeName, err: err}
	}
	_, err = npc.secrets.Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getSecretName(nodeName),
			Namespace: metav1.NamespaceSystem,
		},
		Immutable: ptr.To(true),
		Data:      map[string][]byte{"hash": []byte(hash)},
		Type:      SecretTypeNodePassword,
	})
	if err == nil {
		return nil
	}
	if apierrors.IsAlreadyExists(err) {
		return npc.verifyHash(nodeName, pass, false)
	}
	if vErr := npc.verifyHash(nodeName, pass, false); vErr == nil {
		return nil
	}
	return err
}

func secretNotFound(err error) bool {
	for e := err; e != nil; e = stderrors.Unwrap(e) {
		if apierrors.IsNotFound(e) {
			return true
		}
	}
	return false
}

// verifyNode confirms that a node with the given name exists, to prevent auth
// from succeeding with a client certificate for a node that has been deleted from the cluster.
func (npc *nodePasswordController) verifyNode(ctx context.Context, node *nodeInfo) error {
	if nodeName, isNodeAuth := identifier.NodeIdentity(node.User); isNodeAuth {
		if _, err := npc.nodes.Cache().Get(nodeName); err != nil {
			return errors.WithMessage(err, "unable to verify node identity")
		}
	}
	return nil
}

// Delete uses the controller to delete the secret for a node, if the controller has been started
func Delete(nodeName string) error {
	if controller == nil {
		return util.ErrCoreNotReady
	}
	return controller.secrets.Delete(metav1.NamespaceSystem, getSecretName(nodeName), &metav1.DeleteOptions{})
}
