package server

import (
	"context"
	"time"

	k3scrds "github.com/k3s-io/api/pkg/crds"
	"github.com/k3s-io/api/pkg/generated/controllers/k3s.cattle.io"
	helmcrds "github.com/k3s-io/helm-controller/pkg/crds"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	pkgerrors "github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/apps"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/batch"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/discovery"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac"
	"github.com/rancher/wrangler/v3/pkg/start"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

type Context struct {
	K3s       *k3s.Factory
	Helm      *helm.Factory
	Batch     *batch.Factory
	Apps      *apps.Factory
	Auth      *rbac.Factory
	Core      *core.Factory
	Discovery *discovery.Factory
	K8s       kubernetes.Interface
	Event     record.EventRecorder
}

func (c *Context) Start(ctx context.Context) error {
	return start.All(ctx, 5, c.K3s, c.Helm, c.Apps, c.Auth, c.Batch, c.Core)
}

func NewContext(ctx context.Context, config *Config, forServer bool) (*Context, error) {
	cfg := config.ControlConfig.Runtime.KubeConfigAdmin
	if forServer {
		cfg = config.ControlConfig.Runtime.KubeConfigSupervisor
	}
	restConfig, err := util.GetRESTConfig(cfg)
	if err != nil {
		return nil, err
	}
	restConfig.UserAgent = util.GetUserAgent(version.Program + "-supervisor")

	k8s, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	var recorder record.EventRecorder
	if forServer {
		recorder = util.BuildControllerEventRecorder(k8s, version.Program+"-supervisor", metav1.NamespaceAll)
		if err := registerCrds(ctx, config, restConfig); err != nil {
			return nil, pkgerrors.WithMessage(err, "failed to register CRDs")
		}
	}

	return &Context{
		K3s:       k3s.NewFactoryFromConfigOrDie(restConfig),
		Helm:      helm.NewFactoryFromConfigOrDie(restConfig),
		K8s:       k8s,
		Auth:      rbac.NewFactoryFromConfigOrDie(restConfig),
		Apps:      apps.NewFactoryFromConfigOrDie(restConfig),
		Batch:     batch.NewFactoryFromConfigOrDie(restConfig),
		Core:      core.NewFactoryFromConfigOrDie(restConfig),
		Discovery: discovery.NewFactoryFromConfigOrDie(restConfig),
		Event:     recorder,
	}, nil
}

type crdLister func() ([]*apiextv1.CustomResourceDefinition, error)

func registerCrds(ctx context.Context, config *Config, restConfig *rest.Config) error {
	listers := []crdLister{k3scrds.List}
	if !config.ControlConfig.DisableHelmController {
		listers = append(listers, helmcrds.List)
	}

	crds := []*apiextv1.CustomResourceDefinition{}
	for _, list := range listers {
		l, err := list()
		if err != nil {
			return err
		}
		crds = append(crds, l...)
	}

	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	return crd.BatchCreateCRDs(ctx, client.ApiextensionsV1().CustomResourceDefinitions(), nil, time.Minute, crds)
}
