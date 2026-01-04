package nodepassword

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/k3s-io/k3s/pkg/authenticator/hash"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	pkgerrors "github.com/pkg/errors"
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

func isAlreadyExists(err error) bool {
	for err != nil {
		if apierrors.IsAlreadyExists(err) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
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

// ensure will verify a node-password secret if it exists, otherwise it will create one
func (npc *nodePasswordController) ensure(nodeName, pass string) error {
	err := npc.verifyHash(nodeName, pass, true)
	if apierrors.IsNotFound(err) {
		var hash string
		hash, err = Hasher.CreateHash(pass)
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
		if err != nil && isAlreadyExists(err) {
			// secret already exists, try to verify again without cache
			return npc.verifyHash(nodeName, pass, false)
		}
	}
	return err
}

// verifyNode confirms that a node with the given name exists, to prevent auth
// from succeeding with a client certificate for a node that has been deleted from the cluster.
func (npc *nodePasswordController) verifyNode(ctx context.Context, node *nodeInfo) error {
	if nodeName, isNodeAuth := identifier.NodeIdentity(node.User); isNodeAuth {
		if _, err := npc.nodes.Cache().Get(nodeName); err != nil {
			return pkgerrors.WithMessage(err, "unable to verify node identity")
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
