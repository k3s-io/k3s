package gvk

import (
	"fmt"

	"github.com/rancher/wrangler/pkg/schemes"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Get(obj runtime.Object) (schema.GroupVersionKind, error) {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Kind != "" {
		return gvk, nil
	}

	gvks, _, err := schemes.All.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to find gvk for %v", obj.GetObjectKind())
	}

	return gvks[0], nil
}

func Set(obj runtime.Object) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Kind != "" {
		return nil
	}

	gvks, _, err := schemes.All.ObjectKinds(obj)
	if err != nil {
		return err
	}

	if len(gvks) == 0 {
		return nil
	}

	kind := obj.GetObjectKind()
	kind.SetGroupVersionKind(gvks[0])
	return nil
}
