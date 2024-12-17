package mock

import (
	"github.com/golang/mock/gomock"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//
// Mocks so that we can call Runtime.Core.Core().V1() without a functioning apiserver
//

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
	ConfigMapMock             *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList]
	EndpointsMock             *fake.MockControllerInterface[*v1.Endpoints, *v1.EndpointsList]
	EventMock                 *fake.MockControllerInterface[*v1.Event, *v1.EventList]
	NamespaceMock             *fake.MockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList]
	NodeMock                  *fake.MockNonNamespacedControllerInterface[*v1.Node, *v1.NodeList]
	PersistentVolumeMock      *fake.MockNonNamespacedControllerInterface[*v1.PersistentVolume, *v1.PersistentVolumeList]
	PersistentVolumeClaimMock *fake.MockControllerInterface[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList]
	PodMock                   *fake.MockControllerInterface[*v1.Pod, *v1.PodList]
	SecretMock                *fake.MockControllerInterface[*v1.Secret, *v1.SecretList]
	ServiceMock               *fake.MockControllerInterface[*v1.Service, *v1.ServiceList]
	ServiceAccountMock        *fake.MockControllerInterface[*v1.ServiceAccount, *v1.ServiceAccountList]
}

func NewV1(c *gomock.Controller) *V1Mock {
	return &V1Mock{
		ConfigMapMock:             fake.NewMockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList](c),
		EndpointsMock:             fake.NewMockControllerInterface[*v1.Endpoints, *v1.EndpointsList](c),
		EventMock:                 fake.NewMockControllerInterface[*v1.Event, *v1.EventList](c),
		NamespaceMock:             fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](c),
		NodeMock:                  fake.NewMockNonNamespacedControllerInterface[*v1.Node, *v1.NodeList](c),
		PersistentVolumeMock:      fake.NewMockNonNamespacedControllerInterface[*v1.PersistentVolume, *v1.PersistentVolumeList](c),
		PersistentVolumeClaimMock: fake.NewMockControllerInterface[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList](c),
		PodMock:                   fake.NewMockControllerInterface[*v1.Pod, *v1.PodList](c),
		SecretMock:                fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](c),
		ServiceMock:               fake.NewMockControllerInterface[*v1.Service, *v1.ServiceList](c),
		ServiceAccountMock:        fake.NewMockControllerInterface[*v1.ServiceAccount, *v1.ServiceAccountList](c),
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

func ErrorNotFound(gv, name string) error {
	return apierrors.NewNotFound(schema.ParseGroupResource(gv), name)
}
