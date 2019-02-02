package v1

import (
	"context"
	"sync"

	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"k8s.io/client-go/rest"
)

type (
	contextKeyType        struct{}
	contextClientsKeyType struct{}
)

type Interface interface {
	RESTClient() rest.Interface
	controller.Starter

	NodesGetter
	ServicesGetter
	PodsGetter
}

type Clients struct {
	Interface Interface

	Node    NodeClient
	Service ServiceClient
	Pod     PodClient
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	nodeControllers    map[string]NodeController
	serviceControllers map[string]ServiceController
	podControllers     map[string]PodController
}

func Factory(ctx context.Context, config rest.Config) (context.Context, controller.Starter, error) {
	c, err := NewForConfig(config)
	if err != nil {
		return ctx, nil, err
	}

	cs := NewClientsFromInterface(c)

	ctx = context.WithValue(ctx, contextKeyType{}, c)
	ctx = context.WithValue(ctx, contextClientsKeyType{}, cs)
	return ctx, c, nil
}

func ClientsFrom(ctx context.Context) *Clients {
	return ctx.Value(contextClientsKeyType{}).(*Clients)
}

func From(ctx context.Context) Interface {
	return ctx.Value(contextKeyType{}).(Interface)
}

func NewClients(config rest.Config) (*Clients, error) {
	iface, err := NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return NewClientsFromInterface(iface), nil
}

func NewClientsFromInterface(iface Interface) *Clients {
	return &Clients{
		Interface: iface,

		Node: &nodeClient2{
			iface: iface.Nodes(""),
		},
		Service: &serviceClient2{
			iface: iface.Services(""),
		},
		Pod: &podClient2{
			iface: iface.Pods(""),
		},
	}
}

func NewForConfig(config rest.Config) (Interface, error) {
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = dynamic.NegotiatedSerializer
	}

	restClient, err := restwatch.UnversionedRESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &Client{
		restClient: restClient,

		nodeControllers:    map[string]NodeController{},
		serviceControllers: map[string]ServiceController{},
		podControllers:     map[string]PodController{},
	}, nil
}

func (c *Client) RESTClient() rest.Interface {
	return c.restClient
}

func (c *Client) Sync(ctx context.Context) error {
	return controller.Sync(ctx, c.starters...)
}

func (c *Client) Start(ctx context.Context, threadiness int) error {
	return controller.Start(ctx, threadiness, c.starters...)
}

type NodesGetter interface {
	Nodes(namespace string) NodeInterface
}

func (c *Client) Nodes(namespace string) NodeInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &NodeResource, NodeGroupVersionKind, nodeFactory{})
	return &nodeClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type ServicesGetter interface {
	Services(namespace string) ServiceInterface
}

func (c *Client) Services(namespace string) ServiceInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ServiceResource, ServiceGroupVersionKind, serviceFactory{})
	return &serviceClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type PodsGetter interface {
	Pods(namespace string) PodInterface
}

func (c *Client) Pods(namespace string) PodInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &PodResource, PodGroupVersionKind, podFactory{})
	return &podClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
