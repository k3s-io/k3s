package apply

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func NewClientFactory(config *rest.Config) ClientFactory {
	return func(gvr schema.GroupVersionResource) (dynamic.NamespaceableResourceInterface, error) {
		client, err := dynamic.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		return client.Resource(gvr), nil
	}
}
