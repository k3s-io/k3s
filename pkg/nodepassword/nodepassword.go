package nodepassword

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/authenticator/hash"
	"github.com/k3s-io/k3s/pkg/passwd"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	// Hasher provides the algorithm for generating and verifying hashes
	Hasher          = hash.NewSCrypt()
	ErrVerifyFailed = errVerifyFailed()
)

type passwordError struct {
	node string
	err  error
}

func (e *passwordError) Error() string {
	return fmt.Sprintf("unable to verify password for node %s: %v", e.node, e.err)
}

func (e *passwordError) Is(target error) bool {
	switch target {
	case ErrVerifyFailed:
		return true
	}
	return false
}

func (e *passwordError) Unwrap() error {
	return e.err
}

func errVerifyFailed() error { return &passwordError{} }

func getSecretName(nodeName string) string {
	return strings.ToLower(nodeName + ".node-password." + version.Program)
}

func verifyHash(secretClient coreclient.SecretController, nodeName, pass string) error {
	name := getSecretName(nodeName)
	secret, err := secretClient.Cache().Get(metav1.NamespaceSystem, name)
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

// Ensure will verify a node-password secret if it exists, otherwise it will create one
func Ensure(secretClient coreclient.SecretController, nodeName, pass string) error {
	err := verifyHash(secretClient, nodeName, pass)
	if apierrors.IsNotFound(err) {
		var hash string
		hash, err = Hasher.CreateHash(pass)
		if err != nil {
			return &passwordError{node: nodeName, err: err}
		}
		_, err = secretClient.Create(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getSecretName(nodeName),
				Namespace: metav1.NamespaceSystem,
			},
			Immutable: ptr.To(true),
			Data:      map[string][]byte{"hash": []byte(hash)},
		})
	}
	return err
}

// Delete will remove a node-password secret
func Delete(secretClient coreclient.SecretController, nodeName string) error {
	return secretClient.Delete(metav1.NamespaceSystem, getSecretName(nodeName), &metav1.DeleteOptions{})
}

// MigrateFile moves password file entries to secrets
func MigrateFile(secretClient coreclient.SecretController, nodeClient coreclient.NodeController, passwordFile string) error {
	_, err := os.Stat(passwordFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	passwd, err := passwd.Read(passwordFile)
	if err != nil {
		return err
	}

	nodeNames := []string{}
	nodeList, _ := nodeClient.Cache().List(nil)
	for _, node := range nodeList {
		nodeNames = append(nodeNames, node.Name)
	}
	if len(nodeNames) == 0 {
		nodeNames = append(nodeNames, passwd.Users()...)
	}

	logrus.Infof("Migrating node password entries from '%s'", passwordFile)
	ensured := int64(0)
	start := time.Now()
	for _, nodeName := range nodeNames {
		if pass, ok := passwd.Pass(nodeName); ok {
			if err := Ensure(secretClient, nodeName, pass); err != nil {
				logrus.Warn(errors.Wrapf(err, "error migrating node password entry for node '%s'", nodeName))
			} else {
				ensured++
			}
		}
	}
	logrus.Infof("Migrated %d node password entries in %s", ensured, time.Since(start))
	return os.Remove(passwordFile)
}
