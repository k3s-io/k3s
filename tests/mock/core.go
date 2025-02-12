package mock

import (
	"context"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//
// Mocks so that we can call Runtime.Core.Core().V1() without a functioning apiserver
//

// explicit interface check for core factory mock
var _ config.CoreFactory = &CoreFactoryMock{}

type CoreFactoryMock struct {
	CoreMock *CoreMock
}

func NewCoreFactory(c *gomock.Controller) *CoreFactoryMock {
	return &CoreFactoryMock{
		CoreMock: NewCore(c),
	}
}

func (m *CoreFactoryMock) Core() core.Interface {
	return m.CoreMock
}

func (m *CoreFactoryMock) Sync(ctx context.Context) error {
	return nil
}

func (m *CoreFactoryMock) Start(ctx context.Context, defaultThreadiness int) error {
	return nil
}

// explicit interface check for core mock
var _ core.Interface = &CoreMock{}

type CoreMock struct {
	V1Mock *V1Mock
}

func NewCore(c *gomock.Controller) *CoreMock {
	return &CoreMock{
		V1Mock: NewV1(c),
	}
}

func (m *CoreMock) V1() corev1.Interface {
	return m.V1Mock
}

// explicit interface check for core v1 mock
var _ corev1.Interface = &V1Mock{}

type V1Mock struct {
	ConfigMapMock              *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList]
	ConfigMapCache             *fake.MockCacheInterface[*v1.ConfigMap]
	EndpointsMock              *fake.MockControllerInterface[*v1.Endpoints, *v1.EndpointsList]
	EndpointsCache             *fake.MockCacheInterface[*v1.Endpoints]
	EventMock                  *fake.MockControllerInterface[*v1.Event, *v1.EventList]
	EventCache                 *fake.MockCacheInterface[*v1.Event]
	NamespaceMock              *fake.MockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList]
	NamespaceCache             *fake.MockNonNamespacedCacheInterface[*v1.Namespace]
	NodeMock                   *fake.MockNonNamespacedControllerInterface[*v1.Node, *v1.NodeList]
	NodeCache                  *fake.MockNonNamespacedCacheInterface[*v1.Node]
	PersistentVolumeMock       *fake.MockNonNamespacedControllerInterface[*v1.PersistentVolume, *v1.PersistentVolumeList]
	PersistentVolumeCache      *fake.MockNonNamespacedCacheInterface[*v1.PersistentVolume]
	PersistentVolumeClaimMock  *fake.MockControllerInterface[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList]
	PersistentVolumeClaimCache *fake.MockCacheInterface[*v1.PersistentVolumeClaim]
	PodMock                    *fake.MockControllerInterface[*v1.Pod, *v1.PodList]
	PodCache                   *fake.MockCacheInterface[*v1.Pod]
	SecretMock                 *fake.MockControllerInterface[*v1.Secret, *v1.SecretList]
	SecretCache                *fake.MockCacheInterface[*v1.Secret]
	ServiceMock                *fake.MockControllerInterface[*v1.Service, *v1.ServiceList]
	ServiceCache               *fake.MockCacheInterface[*v1.Service]
	ServiceAccountMock         *fake.MockControllerInterface[*v1.ServiceAccount, *v1.ServiceAccountList]
	ServiceAccountCache        *fake.MockCacheInterface[*v1.ServiceAccount]
}

func NewV1(c *gomock.Controller) *V1Mock {
	return &V1Mock{
		ConfigMapMock:              fake.NewMockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList](c),
		ConfigMapCache:             fake.NewMockCacheInterface[*v1.ConfigMap](c),
		EndpointsMock:              fake.NewMockControllerInterface[*v1.Endpoints, *v1.EndpointsList](c),
		EndpointsCache:             fake.NewMockCacheInterface[*v1.Endpoints](c),
		EventMock:                  fake.NewMockControllerInterface[*v1.Event, *v1.EventList](c),
		EventCache:                 fake.NewMockCacheInterface[*v1.Event](c),
		NamespaceMock:              fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](c),
		NamespaceCache:             fake.NewMockNonNamespacedCacheInterface[*v1.Namespace](c),
		NodeMock:                   fake.NewMockNonNamespacedControllerInterface[*v1.Node, *v1.NodeList](c),
		NodeCache:                  fake.NewMockNonNamespacedCacheInterface[*v1.Node](c),
		PersistentVolumeMock:       fake.NewMockNonNamespacedControllerInterface[*v1.PersistentVolume, *v1.PersistentVolumeList](c),
		PersistentVolumeCache:      fake.NewMockNonNamespacedCacheInterface[*v1.PersistentVolume](c),
		PersistentVolumeClaimMock:  fake.NewMockControllerInterface[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList](c),
		PersistentVolumeClaimCache: fake.NewMockCacheInterface[*v1.PersistentVolumeClaim](c),
		PodMock:                    fake.NewMockControllerInterface[*v1.Pod, *v1.PodList](c),
		PodCache:                   fake.NewMockCacheInterface[*v1.Pod](c),
		SecretMock:                 fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](c),
		SecretCache:                fake.NewMockCacheInterface[*v1.Secret](c),
		ServiceMock:                fake.NewMockControllerInterface[*v1.Service, *v1.ServiceList](c),
		ServiceCache:               fake.NewMockCacheInterface[*v1.Service](c),
		ServiceAccountMock:         fake.NewMockControllerInterface[*v1.ServiceAccount, *v1.ServiceAccountList](c),
		ServiceAccountCache:        fake.NewMockCacheInterface[*v1.ServiceAccount](c),
	}
}

func (m *V1Mock) ConfigMap() corev1.ConfigMapController {
	return m.ConfigMapMock
}

func (m *V1Mock) Endpoints() corev1.EndpointsController {
	return m.EndpointsMock
}

func (m *V1Mock) Event() corev1.EventController {
	return m.EventMock
}

func (m *V1Mock) Namespace() corev1.NamespaceController {
	return m.NamespaceMock
}

func (m *V1Mock) Node() corev1.NodeController {
	return m.NodeMock
}

func (m *V1Mock) PersistentVolume() corev1.PersistentVolumeController {
	return m.PersistentVolumeMock
}

func (m *V1Mock) PersistentVolumeClaim() corev1.PersistentVolumeClaimController {
	return m.PersistentVolumeClaimMock
}

func (m *V1Mock) Pod() corev1.PodController {
	return m.PodMock
}

func (m *V1Mock) Secret() corev1.SecretController {
	return m.SecretMock
}

func (m *V1Mock) Service() corev1.ServiceController {
	return m.ServiceMock
}

func (m *V1Mock) ServiceAccount() corev1.ServiceAccountController {
	return m.ServiceAccountMock
}

// mock secret store interface

type SecretStore struct {
	secrets map[string]map[string]v1.Secret
}

func (m *SecretStore) Create(secret *v1.Secret) (*v1.Secret, error) {
	if m.secrets == nil {
		m.secrets = map[string]map[string]v1.Secret{}
	}
	if _, ok := m.secrets[secret.Namespace]; !ok {
		m.secrets[secret.Namespace] = map[string]v1.Secret{}
	}
	if _, ok := m.secrets[secret.Namespace][secret.Name]; ok {
		return nil, ErrorAlreadyExists("secret", secret.Name)
	}
	m.secrets[secret.Namespace][secret.Name] = *secret
	return secret, nil
}

func (m *SecretStore) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	if m.secrets == nil {
		return ErrorNotFound("secret", name)
	}
	if _, ok := m.secrets[namespace]; !ok {
		return ErrorNotFound("secret", name)
	}
	if _, ok := m.secrets[namespace][name]; !ok {
		return ErrorNotFound("secret", name)
	}
	delete(m.secrets[namespace], name)
	return nil
}

func (m *SecretStore) Get(namespace, name string) (*v1.Secret, error) {
	if m.secrets == nil {
		return nil, ErrorNotFound("secret", name)
	}
	if _, ok := m.secrets[namespace]; !ok {
		return nil, ErrorNotFound("secret", name)
	}
	if secret, ok := m.secrets[namespace][name]; ok {
		return &secret, nil
	}
	return nil, ErrorNotFound("secret", name)
}

// mock node store interface

type NodeStore struct {
	nodes map[string]v1.Node
}

func (m *NodeStore) Create(node *v1.Node) (*v1.Node, error) {
	if m.nodes == nil {
		m.nodes = map[string]v1.Node{}
	}
	if _, ok := m.nodes[node.Name]; ok {
		return nil, ErrorAlreadyExists("node", node.Name)
	}
	m.nodes[node.Name] = *node
	return node, nil
}

func (m *NodeStore) Get(name string) (*v1.Node, error) {
	if m.nodes == nil {
		return nil, ErrorNotFound("node", name)
	}
	if node, ok := m.nodes[name]; ok {
		return &node, nil
	}
	return nil, ErrorNotFound("node", name)
}

func (m *NodeStore) List(ls labels.Selector) ([]v1.Node, error) {
	nodes := []v1.Node{}
	if ls == nil {
		ls = labels.Everything()
	}
	for _, node := range m.nodes {
		if ls.Matches(labels.Set(node.Labels)) {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// utility functions

func ErrorNotFound(gv, name string) error {
	return apierrors.NewNotFound(schema.ParseGroupResource(gv), name)
}

func ErrorAlreadyExists(gv, name string) error {
	return apierrors.NewAlreadyExists(schema.ParseGroupResource(gv), name)
}
