package objectset

import (
	"fmt"
	"reflect"

	"github.com/rancher/norman/types"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type objectKey struct {
	name      string
	namespace string
}

func newObjectKey(obj v1.Object) objectKey {
	return objectKey{
		namespace: obj.GetNamespace(),
		name:      obj.GetName(),
	}
}

func (o objectKey) String() string {
	if o.namespace == "" {
		return o.name
	}
	return fmt.Sprintf("%s/%s", o.namespace, o.name)
}

type objectCollection map[schema.GroupVersionKind]map[objectKey]runtime.Object

func (o objectCollection) add(obj runtime.Object) error {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	objs := o[obj.GetObjectKind().GroupVersionKind()]
	if objs == nil {
		objs = map[objectKey]runtime.Object{}
		o[obj.GetObjectKind().GroupVersionKind()] = objs
	}

	objs[objectKey{
		namespace: metadata.GetNamespace(),
		name:      metadata.GetName(),
	}] = obj

	return nil
}

type ObjectSet struct {
	errs    []error
	objects objectCollection
	nsed    map[schema.GroupVersionKind]bool
	inputs  []runtime.Object
	order   []runtime.Object
}

func NewObjectSet() *ObjectSet {
	return &ObjectSet{
		nsed:    map[schema.GroupVersionKind]bool{},
		objects: objectCollection{},
	}
}

func (o *ObjectSet) AddInput(objs ...runtime.Object) *ObjectSet {
	for _, obj := range objs {
		if obj == nil || reflect.ValueOf(obj).IsNil() {
			continue
		}
		o.inputs = append(o.inputs, obj)
	}
	return o
}

func (o *ObjectSet) Add(objs ...runtime.Object) *ObjectSet {
	for _, obj := range objs {
		o.add(obj)
	}
	return o
}

func (o *ObjectSet) add(obj runtime.Object) {
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	metadata, err := meta.Accessor(obj)
	if err != nil {
		o.err(fmt.Errorf("failed to get metadata for %s", gvk))
		return
	}

	name := metadata.GetName()
	if name == "" {
		o.err(fmt.Errorf("%s is missing name", gvk))
		return
	}

	namespace := metadata.GetNamespace()
	nsed, ok := o.nsed[gvk]
	if ok && nsed != (namespace != "") {
		o.err(fmt.Errorf("got %s objects that are both namespaced and not namespaced", gvk))
		return
	}
	o.nsed[gvk] = namespace != ""

	if err := o.objects.add(obj); err != nil {
		o.err(fmt.Errorf("failed to get metadata for %s", gvk))
		return
	}

	o.order = append(o.order, obj)
}

func (o *ObjectSet) err(err error) error {
	o.errs = append(o.errs, err)
	return o.Err()
}

func (o *ObjectSet) AddErr(err error) {
	o.errs = append(o.errs, err)
}

func (o *ObjectSet) Err() error {
	return types.NewErrors(o.errs...)
}
