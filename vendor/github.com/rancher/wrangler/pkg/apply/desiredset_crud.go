package apply

import (
	"bytes"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
)

var (
	deletePolicy = v1.DeletePropagationBackground
)

func (o *desiredSet) toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	unstruct, ok := obj.(*unstructured.Unstructured)
	if ok {
		return unstruct, nil
	}

	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(obj); err != nil {
		return nil, err
	}

	unstruct = &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	return unstruct, json.Unmarshal(buf.Bytes(), &unstruct.Object)
}

func (o *desiredSet) create(nsed bool, namespace string, client dynamic.NamespaceableResourceInterface, obj runtime.Object) (runtime.Object, error) {
	unstr, err := o.toUnstructured(obj)
	if err != nil {
		return nil, err
	}

	if nsed {
		return client.Namespace(namespace).Create(o.ctx, unstr, v1.CreateOptions{})
	}
	return client.Create(o.ctx, unstr, v1.CreateOptions{})
}

func (o *desiredSet) get(nsed bool, namespace, name string, client dynamic.NamespaceableResourceInterface) (runtime.Object, error) {
	if nsed {
		return client.Namespace(namespace).Get(o.ctx, name, v1.GetOptions{})
	}
	return client.Get(o.ctx, name, v1.GetOptions{})
}

func (o *desiredSet) delete(nsed bool, namespace, name string, client dynamic.NamespaceableResourceInterface, force bool) error {
	if o.noDelete && !force {
		return nil
	}
	opts := v1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}
	if nsed {
		return client.Namespace(namespace).Delete(o.ctx, name, opts)
	}

	return client.Delete(o.ctx, name, opts)
}
