package objectset

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/rancher/wrangler/pkg/merr"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ObjectKey struct {
	Name      string
	Namespace string
}

func NewObjectKey(obj v1.Object) ObjectKey {
	return ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func (o ObjectKey) String() string {
	if o.Namespace == "" {
		return o.Name
	}
	return fmt.Sprintf("%s/%s", o.Namespace, o.Name)
}

type ObjectByGVK map[schema.GroupVersionKind]map[ObjectKey]runtime.Object

func (o ObjectByGVK) Add(obj runtime.Object) error {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	objs := o[obj.GetObjectKind().GroupVersionKind()]
	if objs == nil {
		objs = map[ObjectKey]runtime.Object{}
		o[obj.GetObjectKind().GroupVersionKind()] = objs
	}

	objs[ObjectKey{
		Namespace: metadata.GetNamespace(),
		Name:      metadata.GetName(),
	}] = obj

	return nil
}

type ObjectSet struct {
	errs    []error
	objects ObjectByGVK
	nsed    map[schema.GroupVersionKind]bool
	inputs  []runtime.Object
	order   []runtime.Object
}

func NewObjectSet() *ObjectSet {
	return &ObjectSet{
		nsed:    map[schema.GroupVersionKind]bool{},
		objects: ObjectByGVK{},
	}
}

func (o *ObjectSet) Inputs() []runtime.Object {
	return o.inputs
}

func (o *ObjectSet) ObjectsByGVK() ObjectByGVK {
	return o.objects
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

	if err := o.objects.Add(obj); err != nil {
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
	return merr.NewErrors(o.errs...)
}

func (o *ObjectSet) Len() int {
	return len(o.objects)
}

func (o *ObjectSet) GVKOrder(known ...schema.GroupVersionKind) []schema.GroupVersionKind {
	seen := map[schema.GroupVersionKind]bool{}
	var gvkOrder []schema.GroupVersionKind

	for _, obj := range o.order {
		if seen[obj.GetObjectKind().GroupVersionKind()] {
			continue
		}
		seen[obj.GetObjectKind().GroupVersionKind()] = true
		gvkOrder = append(gvkOrder, obj.GetObjectKind().GroupVersionKind())
	}

	var rest []schema.GroupVersionKind

	for _, gvk := range known {
		if seen[gvk] {
			continue
		}

		seen[gvk] = true
		rest = append(rest, gvk)
	}

	sort.Slice(rest, func(i, j int) bool {
		return rest[i].String() < rest[j].String()
	})

	return append(gvkOrder, rest...)
}
