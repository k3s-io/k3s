package crd

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rancher/wrangler/pkg/kv"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

type Factory struct {
	wg        sync.WaitGroup
	err       error
	CRDClient clientset.Interface
}

type CRD struct {
	GVK          schema.GroupVersionKind
	PluralName   string
	NonNamespace bool
}

func (c CRD) ToCustomResourceDefinition() apiext.CustomResourceDefinition {
	plural := c.PluralName
	if plural == "" {
		plural = strings.ToLower(name.GuessPluralName(c.GVK.Kind))
	}

	name := strings.ToLower(plural + "." + c.GVK.Group)

	crd := apiext.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiext.CustomResourceDefinitionSpec{
			Group: c.GVK.Group,
			Version: c.GVK.Version,
			Versions: []apiext.CustomResourceDefinitionVersion{
				{
					Name:    c.GVK.Version,
					Storage: true,
					Served:  true,
				},
			},
			Names: apiext.CustomResourceDefinitionNames{
				Plural: plural,
				Kind:   c.GVK.Kind,
			},
		},
	}

	if c.NonNamespace {
		crd.Spec.Scope = apiext.ClusterScoped
	} else {
		crd.Spec.Scope = apiext.NamespaceScoped
	}

	return crd
}

func NamespacedType(name string) CRD {
	kindGroup, version := kv.Split(name, "/")
	kind, group := kv.Split(kindGroup, ".")

	return FromGV(schema.GroupVersion{
		Group:   group,
		Version: version,
	}, kind)
}

func NamespacedTypes(names ...string) (ret []CRD) {
	for _, name := range names {
		ret = append(ret, NamespacedType(name))
	}
	return
}

func NonNamespacedType(name string) CRD {
	crd := NamespacedType(name)
	crd.NonNamespace = true
	return crd
}

func NonNamespacedTypes(names ...string) (ret []CRD) {
	for _, name := range names {
		ret = append(ret, NonNamespacedType(name))
	}
	return
}

func FromGV(gv schema.GroupVersion, kind string) CRD {
	return CRD{
		GVK: gv.WithKind(kind),
	}
}

func NewFactoryFromClientGetter(client clientset.Interface) *Factory {
	return &Factory{
		CRDClient: client,
	}
}

func NewFactoryFromClient(config *rest.Config) (*Factory, error) {
	f, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Factory{
		CRDClient: f,
	}, nil
}

func (f *Factory) BatchWait() error {
	f.wg.Wait()
	return f.err
}

func (f *Factory) BatchCreateCRDs(ctx context.Context, crds ...CRD) {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		if _, err := f.CreateCRDs(ctx, crds...); err != nil && f.err == nil {
			f.err = err
		}
	}()
}

func (f *Factory) CreateCRDs(ctx context.Context, crds ...CRD) (map[schema.GroupVersionKind]*apiext.CustomResourceDefinition, error) {
	if len(crds) == 0 {
		return nil, nil
	}

	crdStatus := map[schema.GroupVersionKind]*apiext.CustomResourceDefinition{}

	ready, err := f.getReadyCRDs()
	if err != nil {
		return nil, err
	}

	for _, crdDef := range crds {
		crd, err := f.createCRD(crdDef, ready)
		if err != nil {
			return nil, err
		}
		crdStatus[crdDef.GVK] = crd
	}

	ready, err = f.getReadyCRDs()
	if err != nil {
		return nil, err
	}

	for gvk, crd := range crdStatus {
		if readyCrd, ok := ready[crd.Name]; ok {
			crdStatus[gvk] = readyCrd
		} else {
			if err := f.waitCRD(ctx, crd.Name, gvk, crdStatus); err != nil {
				return nil, err
			}
		}
	}

	return crdStatus, nil
}

func (f *Factory) waitCRD(ctx context.Context, crdName string, gvk schema.GroupVersionKind, crdStatus map[schema.GroupVersionKind]*apiext.CustomResourceDefinition) error {
	logrus.Infof("Waiting for CRD %s to become available", crdName)
	defer logrus.Infof("Done waiting for CRD %s to become available", crdName)

	first := true
	return wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		if !first {
			logrus.Infof("Waiting for CRD %s to become available", crdName)
		}
		first = false

		crd, err := f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(crdName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiext.Established:
				if cond.Status == apiext.ConditionTrue {
					crdStatus[gvk] = crd
					return true, err
				}
			case apiext.NamesAccepted:
				if cond.Status == apiext.ConditionFalse {
					logrus.Infof("Name conflict on %s: %v\n", crdName, cond.Reason)
				}
			}
		}

		return false, ctx.Err()
	})
}

func (f *Factory) createCRD(crdDef CRD, ready map[string]*apiext.CustomResourceDefinition) (*apiext.CustomResourceDefinition, error) {
	plural := crdDef.PluralName
	if plural == "" {
		plural = strings.ToLower(name.GuessPluralName(crdDef.GVK.Kind))
	}

	crd := crdDef.ToCustomResourceDefinition()

	existing, ok := ready[crd.Name]
	if ok {
		if !equality.Semantic.DeepEqual(crd.Spec.Versions, existing.Spec.Versions) {
			existing.Spec = crd.Spec
			logrus.Infof("Updating CRD %s", crd.Name)
			return f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Update(existing)
		}
		return existing, nil
	}

	logrus.Infof("Creating CRD %s", crd.Name)
	return f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(&crd)
}

func (f *Factory) getReadyCRDs() (map[string]*apiext.CustomResourceDefinition, error) {
	list, err := f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := map[string]*apiext.CustomResourceDefinition{}

	for i, crd := range list.Items {
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiext.Established:
				if cond.Status == apiext.ConditionTrue {
					result[crd.Name] = &list.Items[i]
				}
			}
		}
	}

	return result, nil
}
