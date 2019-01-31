package norman

import (
	"context"
	"net/http"

	"github.com/rancher/norman/api"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/types"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ClientFactory func(context.Context, rest.Config) (context.Context, controller.Starter, error)

type ControllerRegister func(ctx context.Context) error

type Config struct {
	Name                  string
	EnableAPI             bool
	Schemas               []*types.Schemas
	CRDs                  map[*types.APIVersion][]string
	Clients               []ClientFactory
	ClientGetter          proxy.ClientGetter
	CRDStorageContext     types.StorageContext
	K8sClient             kubernetes.Interface
	APIExtClient          clientset.Interface
	Config                *rest.Config
	DisableLeaderElection bool
	LeaderLockNamespace   string
	KubeConfig            string
	IgnoredKubeConfigEnv  bool
	Threadiness           int

	CustomizeSchemas func(context.Context, proxy.ClientGetter, *types.Schemas) error
	GlobalSetup      func(context.Context) (context.Context, error)
	MasterSetup      func(context.Context) (context.Context, error)
	PreStart         func(context.Context) error
	APISetup         func(context.Context, *api.Server) error

	PerServerControllers []ControllerRegister
	MasterControllers    []ControllerRegister
}

type Server struct {
	*Config
	*Runtime
}

type Runtime struct {
	AllSchemas        *types.Schemas
	LocalConfig       *rest.Config
	UnversionedClient rest.Interface
	APIHandler        http.Handler
}

type Options struct {
	KubeConfig         string
	DisableControllers bool
}
