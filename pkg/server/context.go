package server

import (
	"context"
	"fmt"
	"time"

	k3scrds "github.com/k3s-io/api/pkg/crds"
	"github.com/k3s-io/api/pkg/generated/controllers/k3s.cattle.io"
	helmcrds "github.com/k3s-io/helm-controller/pkg/crds"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/wrangler/v3/pkg/crd"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/apps"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/batch"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/discovery"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac"
	"github.com/rancher/wrangler/v3/pkg/start"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

type Context struct {
	K3s       *k3s.Factory
	Helm      *helm.Factory
	Batch     *batch.Factory
	Apps      *apps.Factory
	Auth      *rbac.Factory
	Core      *core.Factory
	Discovery *discovery.Factory

	Event record.EventRecorder
	K8s   kubernetes.Interface
	Ext   apiext.Interface
}

func (c *Context) Start(ctx context.Context) error {
	starters := []start.Starter{
		c.K3s, c.Apps, c.Auth, c.Batch, c.Core, c.Discovery,
	}
	if c.Helm != nil {
		starters = append(starters, c.Helm)
	}

	return start.All(ctx, 5, starters...)
}

func NewContext(ctx context.Context, config *Config) (*Context, error) {
	restConfig, err := util.GetRESTConfig(config.ControlConfig.Runtime.KubeConfigSupervisor)
	if err != nil {
		return nil, err
	}
	restConfig.UserAgent = util.GetUserAgent(version.Program + "-supervisor")

	k8s, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	ext, err := apiext.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	var hf *helm.Factory
	if !config.ControlConfig.DisableHelmController {
		hf = helm.NewFactoryFromConfigOrDie(restConfig)
	}

	c := &Context{
		K3s:       k3s.NewFactoryFromConfigOrDie(restConfig),
		Auth:      rbac.NewFactoryFromConfigOrDie(restConfig),
		Apps:      apps.NewFactoryFromConfigOrDie(restConfig),
		Batch:     batch.NewFactoryFromConfigOrDie(restConfig),
		Core:      core.NewFactoryFromConfigOrDie(restConfig),
		Discovery: discovery.NewFactoryFromConfigOrDie(restConfig),
		Helm:      hf,

		Event: util.BuildControllerEventRecorder(k8s, version.Program+"-supervisor", metav1.NamespaceAll),
		K8s:   k8s,
		Ext:   ext,
	}

	if err := c.registerCRDs(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

type crdLister func() ([]*apiextv1.CustomResourceDefinition, error)

func (c *Context) registerCRDs(ctx context.Context) error {
	listers := []crdLister{k3scrds.List}
	if c.Helm != nil {
		listers = append(listers, helmcrds.List)
	}

	crds := []*apiextv1.CustomResourceDefinition{}
	for _, list := range listers {
		l, err := list()
		if err != nil {
			return fmt.Errorf("failed to get CRDs from %s: %v", util.GetFunctionName(list), err)
		}
		crds = append(crds, l...)
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return crd.BatchCreateCRDs(ctx, c.Ext.ApiextensionsV1().CustomResourceDefinitions(), nil, time.Minute, crds)
	})
}
