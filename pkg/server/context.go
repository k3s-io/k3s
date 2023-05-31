package server

import (
	"context"

	helmcrd "github.com/k3s-io/helm-controller/pkg/crd"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	addoncrd "github.com/k3s-io/k3s/pkg/crd"
	"github.com/k3s-io/k3s/pkg/generated/controllers/k3s.cattle.io"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/rancher/wrangler/pkg/crd"
	"github.com/rancher/wrangler/pkg/generated/controllers/apps"
	"github.com/rancher/wrangler/pkg/generated/controllers/batch"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/pkg/generated/controllers/rbac"
	"github.com/rancher/wrangler/pkg/start"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
)

type Context struct {
	K3s   *k3s.Factory
	Helm  *helm.Factory
	Batch *batch.Factory
	Apps  *apps.Factory
	Auth  *rbac.Factory
	Core  *core.Factory
	K8s   kubernetes.Interface
	Event record.EventRecorder
}

func (c *Context) Start(ctx context.Context) error {
	return start.All(ctx, 5, c.K3s, c.Helm, c.Apps, c.Auth, c.Batch, c.Core)
}

func NewContext(ctx context.Context, cfg string) (*Context, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", cfg)
	if err != nil {
		return nil, err
	}
	restConfig.UserAgent = util.GetUserAgent(version.Program + "-supervisor")

	if err := crds(ctx, restConfig); err != nil {
		return nil, errors.Wrap(err, "failed to register CRDs")
	}

	k8s, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return &Context{
		K3s:   k3s.NewFactoryFromConfigOrDie(restConfig),
		Helm:  helm.NewFactoryFromConfigOrDie(restConfig),
		K8s:   k8s,
		Auth:  rbac.NewFactoryFromConfigOrDie(restConfig),
		Apps:  apps.NewFactoryFromConfigOrDie(restConfig),
		Batch: batch.NewFactoryFromConfigOrDie(restConfig),
		Core:  core.NewFactoryFromConfigOrDie(restConfig),
		Event: util.BuildControllerEventRecorder(k8s, version.Program+"-supervisor", metav1.NamespaceAll),
	}, nil
}

func crds(ctx context.Context, config *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(config)
	if err != nil {
		return err
	}

	types := append(helmcrd.List(), addoncrd.List()...)
	factory.BatchCreateCRDs(ctx, types...)

	return factory.BatchWait()
}
