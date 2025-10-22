package nodepassword

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/secretsencrypt"
	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	toolscache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/pager"
)

// MinOrphanSecretAge is the minimum age for an orphaned node-password secret to be cleaned up.
// Since the secret is created when the supervisor reqeusts certs from the server, but the
// node is not created until after containerd and the kubelet start, we need to allow a reasonable
// amount of time before cleaning it up.
var MinOrphanSecretAge = 10 * time.Minute

// controller holds a reference to the last registered nodePasswordController,
// so that the node password validator can share its caches.
var controller *nodePasswordController

func Register(ctx context.Context, coreClient kubernetes.Interface, secrets coreclient.SecretController, nodes coreclient.NodeController) error {
	// start a cache that only watches only node-password secrets in the kube-system namespace
	lw := toolscache.NewListWatchFromClient(coreClient.CoreV1().RESTClient(), "secrets", metav1.NamespaceSystem, fields.OneTermEqualSelector("type", string(SecretTypeNodePassword)))
	informerOpts := toolscache.InformerOptions{ListerWatcher: lw, ObjectType: &corev1.Secret{}, Handler: &toolscache.ResourceEventHandlerFuncs{}}
	indexer, informer := toolscache.NewInformerWithOptions(informerOpts)
	npc := &nodePasswordController{
		nodes:        nodes,
		secrets:      secrets,
		secretsStore: indexer,
	}

	// migrate legacy secrets over to the new type. this must not be fatal, as
	// there may be validating webhooks that prevent deleting or creating secrets
	// until the cluster is up. ref: github.com/k3s-io/k3s/issues/7654
	if err := npc.migrateSecrets(ctx); err != nil {
		logrus.Errorf("Failed to migrate node-password secrets: %v", err)
	}

	nodes.OnChange(ctx, "node-password", npc.onChangeNode)
	go informer.Run(ctx.Done())
	go wait.UntilWithContext(ctx, npc.sync, time.Minute)

	controller = npc
	return nil
}

type nodePasswordController struct {
	nodes        coreclient.NodeController
	secrets      coreclient.SecretController
	secretsStore toolscache.Store
}

// onChangeNode ensures that the node password secret has an OwnerRefence to its
// node, after the node has been created. This will ensure that the garbage
// collector removes the secret once the owning node is deleted.
func (npc *nodePasswordController) onChangeNode(key string, node *corev1.Node) (*corev1.Node, error) {
	if node == nil {
		return node, nil
	}
	secret, err := npc.getSecret(node.Name, true)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return node, nil
		}
		return nil, err
	}
	for _, ref := range secret.OwnerReferences {
		if ref.APIVersion == node.APIVersion && ref.Kind == node.Kind && ref.Name == node.Name && ref.UID == node.UID {
			return node, nil
		}
	}
	logrus.Infof("Adding node OwnerReference to node-password secret %s", secret.Name)
	secret = secret.DeepCopy()
	secret.OwnerReferences = append(secret.OwnerReferences, metav1.OwnerReference{
		APIVersion: node.APIVersion,
		Kind:       node.Kind,
		Name:       node.Name,
		UID:        node.UID,
	})
	_, err = npc.secrets.Update(secret)
	return node, err
}

// sync deletes all node password secrets older than the configured time that
// do not have a corresponding node. Garbage collection should handle secrets
// for nodes that were deleted, so this cleanup is mostly for nodes that
// requested certificates but never successfully joined the cluster.
func (npc *nodePasswordController) sync(ctx context.Context) {
	if !npc.nodes.Informer().HasSynced() {
		return
	}
	minCreateTime := time.Now().Add(-MinOrphanSecretAge)
	nodeSecretNames := sets.Set[string]{}
	for _, nodeName := range npc.nodes.Informer().GetStore().ListKeys() {
		nodeSecretNames = nodeSecretNames.Insert(getSecretName(nodeName))
	}
	for _, s := range npc.secretsStore.List() {
		secret, ok := s.(*corev1.Secret)
		if !ok || secret.CreationTimestamp.After(minCreateTime) || nodeSecretNames.Has(secret.Name) {
			continue
		}
		if err := npc.secrets.Delete(secret.Namespace, secret.Name, &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &secret.UID}}); err != nil {
			logrus.Errorf("Failed to delete orphaned node-password secret %s: %v", secret.Name, err)
		} else {
			logrus.Warnf("Deleted orphaned node-password secret %s created %s", secret.Name, secret.CreationTimestamp)
		}
	}
}

// migrateSecrets recreates legacy node password secrets with the correct type
func (npc *nodePasswordController) migrateSecrets(ctx context.Context) error {
	secretSuffix := getSecretName("")
	secretPager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		return npc.secrets.List(metav1.NamespaceSystem, opts)
	}))
	secretPager.PageSize = secretsencrypt.SecretListPageSize

	return secretPager.EachListItem(ctx, metav1.ListOptions{}, func(obj runtime.Object) error {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return errors.New("failed to convert object to Secret")
		}
		// skip migrating secrets that already have the correct type, or are not a node password secret
		if secret.Type == SecretTypeNodePassword || !strings.HasSuffix(secret.Name, secretSuffix) {
			return nil
		}

		// delete the old object, and create a new one with the correct type -
		// we have to delete and re-create because the type field is immutable
		logrus.Infof("Migrating node-password secret %s", secret.Name)
		deleteOpts := &metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &secret.ObjectMeta.UID}}
		if err := npc.secrets.Delete(secret.Namespace, secret.Name, deleteOpts); err != nil && !apierrors.IsConflict(err) && !apierrors.IsNotFound(err) {
			return err
		}
		newSecret := secret.DeepCopy()
		newSecret.ObjectMeta.UID = ""
		newSecret.ObjectMeta.ResourceVersion = ""
		newSecret.Type = SecretTypeNodePassword
		if _, err := npc.secrets.Create(newSecret); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		return nil
	})
}

// getSecret is a helper function to get a node password secret from the store,
// or directly from the apiserver.
func (npc *nodePasswordController) getSecret(nodeName string, cached bool) (*corev1.Secret, error) {
	if cached {
		name := metav1.NamespaceSystem + "/" + getSecretName(nodeName)
		val, ok, err := npc.secretsStore.GetByKey(name)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secret"}, name)
		}
		s, ok := val.(*corev1.Secret)
		if !ok {
			return nil, errors.New("failed to convert object to Secret")
		}
		return s, nil
	}
	return npc.secrets.Get(metav1.NamespaceSystem, getSecretName(nodeName), metav1.GetOptions{})
}
