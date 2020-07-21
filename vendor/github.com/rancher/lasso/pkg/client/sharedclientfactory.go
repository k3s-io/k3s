package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/rancher/lasso/pkg/mapper"
	"github.com/rancher/lasso/pkg/scheme"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
)

type SharedClientFactoryOptions struct {
	Mapper meta.RESTMapper
	Scheme *runtime.Scheme
}

type SharedClientFactory interface {
	ForKind(gvk schema.GroupVersionKind) (*Client, error)
	ForResource(gvr schema.GroupVersionResource, namespaced bool) (*Client, error)
	ForResourceKind(gvr schema.GroupVersionResource, kind string, namespaced bool) *Client
	NewObjects(gvk schema.GroupVersionKind) (runtime.Object, runtime.Object, error)
	GVKForObject(obj runtime.Object) (schema.GroupVersionKind, error)
	GVKForResource(gvr schema.GroupVersionResource) (schema.GroupVersionKind, error)
	ResourceForGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool, error)
}

type sharedClientFactory struct {
	createLock sync.RWMutex
	clients    map[schema.GroupVersionResource]*Client
	timeout    time.Duration
	rest       rest.Interface

	Mapper meta.RESTMapper
	Scheme *runtime.Scheme
}

func NewSharedClientFactoryForConfig(config *rest.Config) (SharedClientFactory, error) {
	return NewSharedClientFactory(config, nil)
}

func NewSharedClientFactory(config *rest.Config, opts *SharedClientFactoryOptions) (_ SharedClientFactory, err error) {
	opts, err = applyDefaults(config, opts)
	if err != nil {
		return nil, err
	}

	config, timeout := populateConfig(opts.Scheme, config)
	rest, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, err
	}

	return &sharedClientFactory{
		timeout: timeout,
		clients: map[schema.GroupVersionResource]*Client{},
		Scheme:  opts.Scheme,
		Mapper:  opts.Mapper,
		rest:    rest,
	}, nil
}

func applyDefaults(config *rest.Config, opts *SharedClientFactoryOptions) (*SharedClientFactoryOptions, error) {
	var newOpts SharedClientFactoryOptions
	if opts != nil {
		newOpts = *opts
	}

	if newOpts.Scheme == nil {
		newOpts.Scheme = scheme.All
	}

	if newOpts.Mapper == nil {
		mapperOpt, err := mapper.New(config)
		if err != nil {
			return nil, err
		}
		newOpts.Mapper = mapperOpt
	}

	return &newOpts, nil
}

func (s *sharedClientFactory) GVKForResource(gvr schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return s.Mapper.KindFor(gvr)
}

func (s *sharedClientFactory) ResourceForGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, bool, error) {
	mapping, err := s.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}

	nsed, err := IsNamespaced(mapping.Resource, s.Mapper)
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}

	return mapping.Resource, nsed, nil
}

func (s *sharedClientFactory) GVKForObject(obj runtime.Object) (schema.GroupVersionKind, error) {
	gvks, _, err := s.Scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to find schema.GroupVersionKind for %T", obj)
	}
	return gvks[0], nil
}

func (s *sharedClientFactory) NewObjects(gvk schema.GroupVersionKind) (runtime.Object, runtime.Object, error) {
	obj, err := s.Scheme.New(gvk)
	if err != nil {
		return nil, nil, err
	}

	objList, err := s.Scheme.New(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind + "List",
	})
	return obj, objList, err
}

func (s *sharedClientFactory) ForKind(gvk schema.GroupVersionKind) (*Client, error) {
	gvr, nsed, err := s.ResourceForGVK(gvk)
	if err != nil {
		return nil, err
	}

	return s.ForResourceKind(gvr, gvk.Kind, nsed), nil
}

func (s *sharedClientFactory) ForResource(gvr schema.GroupVersionResource, namespaced bool) (*Client, error) {
	gvk, err := s.GVKForResource(gvr)
	if err != nil {
		return nil, err
	}
	return s.ForResourceKind(gvr, gvk.Kind, namespaced), nil
}

func (s *sharedClientFactory) ForResourceKind(gvr schema.GroupVersionResource, kind string, namespaced bool) *Client {
	client := s.getClient(gvr)
	if client != nil {
		return client
	}

	s.createLock.Lock()
	defer s.createLock.Unlock()

	client = s.clients[gvr]
	if client != nil {
		return client
	}

	client = NewClient(gvr, kind, namespaced, s.rest, s.timeout)

	s.clients[gvr] = client
	return client
}

func (s *sharedClientFactory) getClient(gvr schema.GroupVersionResource) *Client {
	s.createLock.RLock()
	defer s.createLock.RUnlock()
	return s.clients[gvr]
}

func populateConfig(scheme *runtime.Scheme, config *rest.Config) (*rest.Config, time.Duration) {
	config = rest.CopyConfig(config)
	config.NegotiatedSerializer = serializer.NewCodecFactory(scheme).WithoutConversion()
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	timeout := config.Timeout
	config.Timeout = 0
	return config, timeout
}
