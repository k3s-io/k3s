package server

import (
	"context"

	"github.com/rancher/k3s/pkg/generated/controllers/apps"
	"github.com/rancher/k3s/pkg/generated/controllers/core"
	"github.com/rancher/k3s/pkg/generated/controllers/k3s.cattle.io"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/crd"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/staging/src/k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/staging/src/k8s.io/client-go/tools/clientcmd"
)

type Context struct {
	K3s   *k3s.Factory
	Apps  *apps.Factory
	Core  *core.Factory
	K8s   kubernetes.Interface
	Apply apply.Apply
}

func (c *Context) Start(ctx context.Context) error {
	return start.All(ctx, 5, c.K3s, c.Apps, c.Core)
}

func newContext(ctx context.Context, cfg string) (*Context, error) {
	restConfig, err := clientcmd.BuildConfigFromFlags("", cfg)
	if err != nil {
		return nil, err
	}

	if err := crds(ctx, restConfig); err != nil {
		return nil, err
	}

	k8s := kubernetes.NewForConfigOrDie(restConfig)
	return &Context{
		K3s:   k3s.NewFactoryFromConfigOrDie(restConfig),
		K8s:   k8s,
		Apps:  apps.NewFactoryFromConfigOrDie(restConfig),
		Core:  core.NewFactoryFromConfigOrDie(restConfig),
		Apply: apply.New(k8s, apply.NewClientFactory(restConfig)),
	}, nil
}

func crds(ctx context.Context, config *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(config)
	if err != nil {
		return err
	}

	factory.BatchCreateCRDs(ctx, crd.NamespacedTypes(
		"ListenerConfig.k3s.cattle.io/v1",
		"Addon.k3s.cattle.io/v1",
		"HelmChart.k3s.cattle.io/v1")...)

	return factory.BatchWait()
}
