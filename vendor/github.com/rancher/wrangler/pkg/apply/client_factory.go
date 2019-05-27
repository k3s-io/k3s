package apply

import (
	"strings"

	"github.com/rancher/wrangler/pkg/name"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func NewClientFactory(config *rest.Config) ClientFactory {
	return func(gvk schema.GroupVersionKind) (dynamic.NamespaceableResourceInterface, error) {
		client, err := dynamic.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		gvr := gvk.GroupVersion().WithResource(name.GuessPluralName(strings.ToLower(gvk.Kind)))
		return client.Resource(gvr), nil
	}
}
