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
	ConfigMapCache             *fake.MockCacheInterface[*v1.ConfigMap]
	ConfigMapMock              *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList]
	EndpointsCache             *fake.MockCacheInterface[*v1.Endpoints]
	EndpointsMock              *fake.MockControllerInterface[*v1.Endpoints, *v1.EndpointsList]
	EventCache                 *fake.MockCacheInterface[*v1.Event]
	EventMock                  *fake.MockControllerInterface[*v1.Event, *v1.EventList]
	LimitRangeCache            *fake.MockCacheInterface[*v1.LimitRange]
	LimitRangeMock             *fake.MockControllerInterface[*v1.LimitRange, *v1.LimitRangeList]
	NamespaceCache             *fake.MockNonNamespacedCacheInterface[*v1.Namespace]
	NamespaceMock              *fake.MockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList]
	NodeCache                  *fake.MockNonNamespacedCacheInterface[*v1.Node]
	NodeMock                   *fake.MockNonNamespacedControllerInterface[*v1.Node, *v1.NodeList]
	PersistentVolumeCache      *fake.MockNonNamespacedCacheInterface[*v1.PersistentVolume]
	PersistentVolumeClaimCache *fake.MockCacheInterface[*v1.PersistentVolumeClaim]
	PersistentVolumeClaimMock  *fake.MockControllerInterface[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList]
	PersistentVolumeMock       *fake.MockNonNamespacedControllerInterface[*v1.PersistentVolume, *v1.PersistentVolumeList]
	PodCache                   *fake.MockCacheInterface[*v1.Pod]
	PodMock                    *fake.MockControllerInterface[*v1.Pod, *v1.PodList]
	ResourceQuotaCache         *fake.MockCacheInterface[*v1.ResourceQuota]
	ResourceQuotaMock          *fake.MockControllerInterface[*v1.ResourceQuota, *v1.ResourceQuotaList]
	SecretCache                *fake.MockCacheInterface[*v1.Secret]
	SecretMock                 *fake.MockControllerInterface[*v1.Secret, *v1.SecretList]
	ServiceAccountCache        *fake.MockCacheInterface[*v1.ServiceAccount]
	ServiceAccountMock         *fake.MockControllerInterface[*v1.ServiceAccount, *v1.ServiceAccountList]
	ServiceCache               *fake.MockCacheInterface[*v1.Service]
	ServiceMock                *fake.MockControllerInterface[*v1.Service, *v1.ServiceList]
}

func NewV1(c *gomock.Controller) *V1Mock {
	return &V1Mock{
		ConfigMapCache:             fake.NewMockCacheInterface[*v1.ConfigMap](c),
		ConfigMapMock:              fake.NewMockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList](c),
		EndpointsCache:             fake.NewMockCacheInterface[*v1.Endpoints](c),
		EndpointsMock:              fake.NewMockControllerInterface[*v1.Endpoints, *v1.EndpointsList](c),
		EventCache:                 fake.NewMockCacheInterface[*v1.Event](c),
		EventMock:                  fake.NewMockControllerInterface[*v1.Event, *v1.EventList](c),
		LimitRangeCache:            fake.NewMockCacheInterface[*v1.LimitRange](c),
		LimitRangeMock:             fake.NewMockControllerInterface[*v1.LimitRange, *v1.LimitRangeList](c),
		NamespaceCache:             fake.NewMockNonNamespacedCacheInterface[*v1.Namespace](c),
		NamespaceMock:              fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](c),
		NodeCache:                  fake.NewMockNonNamespacedCacheInterface[*v1.Node](c),
		NodeMock:                   fake.NewMockNonNamespacedControllerInterface[*v1.Node, *v1.NodeList](c),
		PersistentVolumeCache:      fake.NewMockNonNamespacedCacheInterface[*v1.PersistentVolume](c),
		PersistentVolumeClaimCache: fake.NewMockCacheInterface[*v1.PersistentVolumeClaim](c),
		PersistentVolumeClaimMock:  fake.NewMockControllerInterface[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList](c),
		PersistentVolumeMock:       fake.NewMockNonNamespacedControllerInterface[*v1.PersistentVolume, *v1.PersistentVolumeList](c),
		PodCache:                   fake.NewMockCacheInterface[*v1.Pod](c),
		PodMock:                    fake.NewMockControllerInterface[*v1.Pod, *v1.PodList](c),
		ResourceQuotaCache:         fake.NewMockCacheInterface[*v1.ResourceQuota](c),
		ResourceQuotaMock:          fake.NewMockControllerInterface[*v1.ResourceQuota, *v1.ResourceQuotaList](c),
		SecretCache:                fake.NewMockCacheInterface[*v1.Secret](c),
		SecretMock:                 fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](c),
		ServiceAccountCache:        fake.NewMockCacheInterface[*v1.ServiceAccount](c),
		ServiceAccountMock:         fake.NewMockControllerInterface[*v1.ServiceAccount, *v1.ServiceAccountList](c),
		ServiceCache:               fake.NewMockCacheInterface[*v1.Service](c),
		ServiceMock:                fake.NewMockControllerInterface[*v1.Service, *v1.ServiceList](c),
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

func (m *V1Mock) LimitRange() corev1.LimitRangeController {
	return m.LimitRangeMock
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

func (m *V1Mock) ResourceQuota() corev1.ResourceQuotaController {
	return m.ResourceQuotaMock
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

func (m *SecretStore) GetWithOptions(namespace, name string, opts metav1.GetOptions) (*v1.Secret, error) {
	return m.Get(namespace, name)
}

func (m *SecretStore) List(namespace string, ls labels.Selector) ([]v1.Secret, error) {
	secrets := []v1.Secret{}
	if ls == nil {
		ls = labels.Everything()
	}
	for _, secret := range m.secrets[namespace] {
		if ls.Matches(labels.Set(secret.Labels)) {
			secrets = append(secrets, secret)
		}
	}
	return secrets, nil
}

func (m *SecretStore) ListWithOptions(namespace string, opts metav1.ListOptions) (*v1.SecretList, error) {
	ls, err := labels.Parse(opts.LabelSelector)
	if err != nil {
		return nil, err
	}
	secrets, err := m.List(namespace, ls)
	if err != nil {
		return nil, err
	}
	return &v1.SecretList{Items: secrets}, nil
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
