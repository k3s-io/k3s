package apply

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/sirupsen/logrus"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	types2 "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

var (
	ErrReplace      = errors.New("replace object with changes")
	ReplaceOnChange = func(name string, o runtime.Object, patchType types2.PatchType, data []byte, subresources ...string) (runtime.Object, error) {
		return nil, ErrReplace
	}
)

func (o *desiredSet) getControllerAndClient(debugID string, gvk schema.GroupVersionKind) (cache.SharedIndexInformer, dynamic.NamespaceableResourceInterface, error) {
	informer, ok := o.pruneTypes[gvk]
	if !ok {
		informer = o.a.informers[gvk]
	}
	if informer == nil && o.strictCaching {
		return nil, nil, fmt.Errorf("failed to find informer for %s for %s", gvk, debugID)
	}

	client, err := o.a.clients.client(gvk)
	if err != nil {
		return nil, nil, err
	}

	return informer, client, nil
}

func (o *desiredSet) adjustNamespace(gvk schema.GroupVersionKind, objs map[objectset.ObjectKey]runtime.Object) error {
	for k, v := range objs {
		if k.Namespace != "" {
			continue
		}

		v = v.DeepCopyObject()
		meta, err := meta.Accessor(v)
		if err != nil {
			return err
		}

		meta.SetNamespace(o.defaultNamespace)

		delete(objs, k)
		k.Namespace = o.defaultNamespace
		objs[k] = v
	}

	return nil
}

func (o *desiredSet) createPatcher(client dynamic.NamespaceableResourceInterface) Patcher {
	return func(namespace, name string, pt types2.PatchType, data []byte) (object runtime.Object, e error) {
		if namespace != "" {
			return client.Namespace(namespace).Patch(name, pt, data, v1.PatchOptions{})
		}
		return client.Patch(name, pt, data, v1.PatchOptions{})
	}
}

func (o *desiredSet) process(debugID string, set labels.Selector, gvk schema.GroupVersionKind, objs map[objectset.ObjectKey]runtime.Object) {
	controller, client, err := o.getControllerAndClient(debugID, gvk)
	if err != nil {
		o.err(err)
		return
	}

	nsed := o.a.clients.IsNamespaced(gvk)

	if nsed {
		if err := o.adjustNamespace(gvk, objs); err != nil {
			o.err(err)
			return
		}
	}

	patcher, ok := o.patchers[gvk]
	if !ok {
		patcher = o.createPatcher(client)
	}

	existing, err := list(controller, client, set)
	if err != nil {
		o.err(fmt.Errorf("failed to list %s for %s", gvk, debugID))
		return
	}

	toCreate, toDelete, toUpdate := compareSets(existing, objs)

	createF := func(k objectset.ObjectKey) {
		obj := objs[k]
		obj, err := prepareObjectForCreate(obj)
		if err != nil {
			o.err(errors.Wrapf(err, "failed to prepare create %s %s for %s", k, gvk, debugID))
			return
		}

		_, err = o.create(nsed, k.Namespace, client, obj)
		if errors2.IsAlreadyExists(err) {
			// Taking over an object that wasn't previously managed by us
			existingObj, err := o.get(nsed, k.Namespace, k.Name, client)
			if err == nil {
				toUpdate = append(toUpdate, k)
				existing[k] = existingObj
				return
			}
		}
		if err != nil {
			o.err(errors.Wrapf(err, "failed to create %s %s for %s", k, gvk, debugID))
			return
		}
		logrus.Debugf("DesiredSet - Created %s %s for %s", gvk, k, debugID)
	}

	deleteF := func(k objectset.ObjectKey) {
		if err := o.delete(nsed, k.Namespace, k.Name, client); err != nil {
			o.err(errors.Wrapf(err, "failed to delete %s %s for %s", k, gvk, debugID))
			return
		}
		logrus.Debugf("DesiredSet - Delete %s %s for %s", gvk, k, debugID)
	}

	updateF := func(k objectset.ObjectKey) {
		err := o.compareObjects(gvk, patcher, client, debugID, existing[k], objs[k], len(toCreate) > 0 || len(toDelete) > 0)
		if err == ErrReplace {
			deleteF(k)
			o.err(fmt.Errorf("DesiredSet - Replace Wait %s %s for %s", gvk, k, debugID))
		} else if err != nil {
			o.err(errors.Wrapf(err, "failed to update %s %s for %s", k, gvk, debugID))
		}
	}

	for _, k := range toCreate {
		createF(k)
	}

	for _, k := range toUpdate {
		updateF(k)
	}

	for _, k := range toDelete {
		deleteF(k)
	}
}

func compareSets(existingSet, newSet map[objectset.ObjectKey]runtime.Object) (toCreate, toDelete, toUpdate []objectset.ObjectKey) {
	for k := range newSet {
		if _, ok := existingSet[k]; ok {
			toUpdate = append(toUpdate, k)
		} else {
			toCreate = append(toCreate, k)
		}
	}

	for k := range existingSet {
		if _, ok := newSet[k]; !ok {
			toDelete = append(toDelete, k)
		}
	}

	sortObjectKeys(toCreate)
	sortObjectKeys(toDelete)
	sortObjectKeys(toUpdate)

	return
}

func sortObjectKeys(keys []objectset.ObjectKey) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})
}

func addObjectToMap(objs map[objectset.ObjectKey]runtime.Object, obj interface{}) error {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	objs[objectset.ObjectKey{
		Namespace: metadata.GetNamespace(),
		Name:      metadata.GetName(),
	}] = obj.(runtime.Object)

	return nil
}

func list(informer cache.SharedIndexInformer, client dynamic.NamespaceableResourceInterface, selector labels.Selector) (map[objectset.ObjectKey]runtime.Object, error) {
	var (
		errs []error
		objs = map[objectset.ObjectKey]runtime.Object{}
	)

	if informer == nil {
		list, err := client.List(v1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return nil, err
		}

		for _, obj := range list.Items {
			copy := obj
			if err := addObjectToMap(objs, &copy); err != nil {
				errs = append(errs, err)
			}
		}

		return objs, merr.NewErrors(errs...)
	}

	err := cache.ListAllByNamespace(informer.GetIndexer(), "", selector, func(obj interface{}) {
		if err := addObjectToMap(objs, obj); err != nil {
			errs = append(errs, err)
		}
	})
	if err != nil {
		errs = append(errs, err)
	}

	return objs, merr.NewErrors(errs...)
}
