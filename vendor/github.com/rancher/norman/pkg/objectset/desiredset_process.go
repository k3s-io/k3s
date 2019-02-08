package objectset

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/rancher/norman/controller"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/objectclient/dynamic"
	"github.com/rancher/norman/restwatch"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	types2 "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

var (
	deletePolicy    = v1.DeletePropagationBackground
	ErrReplace      = errors.New("replace object with changes")
	ReplaceOnChange = func(name string, o runtime.Object, patchType types2.PatchType, data []byte, subresources ...string) (runtime.Object, error) {
		return nil, ErrReplace
	}
)

func NewDiscoveredClient(gvk schema.GroupVersionKind, restConfig rest.Config, discovery discovery.DiscoveryInterface) (*objectclient.ObjectClient, error) {
	resources, err := discovery.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return nil, err
	}

	for _, resource := range resources.APIResources {
		if resource.Kind != gvk.Kind {
			continue
		}

		if restConfig.NegotiatedSerializer == nil {
			restConfig.NegotiatedSerializer = dynamic.NegotiatedSerializer
		}

		restClient, err := restwatch.UnversionedRESTClientFor(&restConfig)
		if err != nil {
			return nil, err
		}

		objectClient := objectclient.NewObjectClient("", restClient, &resource, gvk, &objectclient.UnstructuredObjectFactory{})
		return objectClient, nil
	}

	return nil, fmt.Errorf("failed to discover client for %s", gvk)
}

func (o *DesiredSet) getControllerAndObjectClient(debugID string, gvk schema.GroupVersionKind) (controller.GenericController, *objectclient.ObjectClient, error) {
	client, ok := o.clients[gvk]
	if !ok && o.discovery == nil {
		return nil, nil, fmt.Errorf("failed to find client for %s for %s", gvk, debugID)
	}

	if client != nil {
		return client.Generic(), client.ObjectClient(), nil
	}

	objectClient := o.discoveredClients[gvk]
	if objectClient != nil {
		return nil, objectClient, nil
	}

	objectClient, err := NewDiscoveredClient(gvk, o.restConfig, o.discovery)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to find client for %s for %s", gvk, debugID)
	}

	o.AddDiscoveredClient(gvk, objectClient)
	return nil, objectClient, nil
}

func (o *DesiredSet) process(inputID, debugID string, set labels.Selector, gvk schema.GroupVersionKind, objs map[objectKey]runtime.Object) {
	controller, objectClient, err := o.getControllerAndObjectClient(debugID, gvk)
	if err != nil {
		o.err(err)
		return
	}

	patcher, ok := o.patchers[gvk]
	if !ok {
		patcher = objectClient.Patch
	}

	existing, err := list(controller, objectClient, set)
	if err != nil {
		o.err(fmt.Errorf("failed to list %s for %s", gvk, debugID))
		return
	}

	toCreate, toDelete, toUpdate := compareSets(existing, objs)

	createF := func(k objectKey) {
		obj := objs[k]
		obj, err := prepareObjectForCreate(inputID, obj)
		if err != nil {
			o.err(errors.Wrapf(err, "failed to prepare create %s %s for %s", k, gvk, debugID))
			return
		}

		_, err = objectClient.Create(obj)
		if errors2.IsAlreadyExists(err) {
			// Taking over an object that wasn't previously managed by us
			existingObj, err := objectClient.GetNamespaced(k.namespace, k.name, v1.GetOptions{})
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

	deleteF := func(k objectKey) {
		err := objectClient.DeleteNamespaced(k.namespace, k.name, &v1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})
		if err != nil {
			o.err(errors.Wrapf(err, "failed to delete %s %s for %s", k, gvk, debugID))
			return
		}
		logrus.Debugf("DesiredSet - Delete %s %s for %s", gvk, k, debugID)
	}

	updateF := func(k objectKey) {
		err := o.compareObjects(patcher, objectClient, debugID, inputID, existing[k], objs[k], len(toCreate) > 0 || len(toDelete) > 0)
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

func compareSets(existingSet, newSet map[objectKey]runtime.Object) (toCreate, toDelete, toUpdate []objectKey) {
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

func sortObjectKeys(keys []objectKey) {
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})
}

func addObjectToMap(objs map[objectKey]runtime.Object, obj interface{}) error {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	objs[objectKey{
		namespace: metadata.GetNamespace(),
		name:      metadata.GetName(),
	}] = obj.(runtime.Object)

	return nil
}

func list(controller controller.GenericController, objectClient *objectclient.ObjectClient, selector labels.Selector) (map[objectKey]runtime.Object, error) {
	var (
		errs []error
		objs = map[objectKey]runtime.Object{}
	)

	if controller == nil {
		objList, err := objectClient.List(v1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return nil, err
		}

		list, ok := objList.(*unstructured.UnstructuredList)
		if !ok {
			return nil, fmt.Errorf("invalid list type %T", objList)
		}

		for _, obj := range list.Items {
			if err := addObjectToMap(objs, &obj); err != nil {
				errs = append(errs, err)
			}
		}

		return objs, types.NewErrors(errs...)
	}

	err := cache.ListAllByNamespace(controller.Informer().GetIndexer(), "", selector, func(obj interface{}) {
		if err := addObjectToMap(objs, obj); err != nil {
			errs = append(errs, err)
		}
	})
	if err != nil {
		errs = append(errs, err)
	}

	return objs, types.NewErrors(errs...)
}
