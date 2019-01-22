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

	ListenerConfigsGetter
	AddonsGetter
}

type Clients struct {
	Interface Interface

	ListenerConfig ListenerConfigClient
	Addon          AddonClient
}

type Client struct {
	sync.Mutex
	restClient rest.Interface
	starters   []controller.Starter

	listenerConfigControllers map[string]ListenerConfigController
	addonControllers          map[string]AddonController
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

		ListenerConfig: &listenerConfigClient2{
			iface: iface.ListenerConfigs(""),
		},
		Addon: &addonClient2{
			iface: iface.Addons(""),
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

		listenerConfigControllers: map[string]ListenerConfigController{},
		addonControllers:          map[string]AddonController{},
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

type ListenerConfigsGetter interface {
	ListenerConfigs(namespace string) ListenerConfigInterface
}

func (c *Client) ListenerConfigs(namespace string) ListenerConfigInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &ListenerConfigResource, ListenerConfigGroupVersionKind, listenerConfigFactory{})
	return &listenerConfigClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}

type AddonsGetter interface {
	Addons(namespace string) AddonInterface
}

func (c *Client) Addons(namespace string) AddonInterface {
	objectClient := objectclient.NewObjectClient(namespace, c.restClient, &AddonResource, AddonGroupVersionKind, addonFactory{})
	return &addonClient{
		ns:           namespace,
		client:       c,
		objectClient: objectClient,
	}
}
