package crd

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/kv"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/schemas/openapi"
	"github.com/sirupsen/logrus"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	Schema       *v1beta1.JSONSchemaProps
	SchemaObject interface{}
	Columns      []v1beta1.CustomResourceColumnDefinition
	Status       bool
	Scale        bool
	Categories   []string
	ShortNames   []string
}

func (c CRD) WithSchema(schema *v1beta1.JSONSchemaProps) CRD {
	c.Schema = schema
	return c
}

func (c CRD) WithSchemaFromStruct(obj interface{}) CRD {
	c.SchemaObject = obj
	return c
}

func (c CRD) WithColumn(name, path string) CRD {
	c.Columns = append(c.Columns, v1beta1.CustomResourceColumnDefinition{
		Name:     name,
		Type:     "string",
		Priority: 0,
		JSONPath: path,
	})
	return c
}

func getType(obj interface{}) reflect.Type {
	if t, ok := obj.(reflect.Type); ok {
		return t
	}

	t := reflect.TypeOf(obj)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func (c CRD) WithColumnsFromStruct(obj interface{}) CRD {
	c.Columns = append(c.Columns, readCustomColumns(getType(obj), ".")...)
	return c
}

func fieldName(f reflect.StructField) string {
	jsonTag := f.Tag.Get("json")
	if jsonTag == "-" {
		return ""
	}
	name := strings.Split(jsonTag, ",")[0]
	if name == "" {
		return f.Name
	}
	return name
}

func tagToColumn(f reflect.StructField) (v1beta1.CustomResourceColumnDefinition, bool) {
	c := v1beta1.CustomResourceColumnDefinition{
		Name: f.Name,
		Type: "string",
	}

	columnDef, ok := f.Tag.Lookup("column")
	if !ok {
		return c, false
	}

	for k, v := range kv.SplitMap(columnDef, ",") {
		switch k {
		case "name":
			c.Name = v
		case "type":
			c.Type = v
		case "format":
			c.Format = v
		case "description":
			c.Description = v
		case "priority":
			p, _ := strconv.Atoi(v)
			c.Priority = int32(p)
		case "jsonpath":
			c.JSONPath = v
		}
	}

	return c, true
}

func readCustomColumns(t reflect.Type, path string) (result []v1beta1.CustomResourceColumnDefinition) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fieldName := fieldName(f)
		if fieldName == "" {
			continue
		}

		t := f.Type
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() == reflect.Struct {
			if f.Anonymous {
				result = append(result, readCustomColumns(t, path)...)
			} else {
				result = append(result, readCustomColumns(t, path+"."+fieldName)...)
			}
		} else {
			if col, ok := tagToColumn(f); ok {
				result = append(result, col)
			}
		}
	}

	return result
}

func (c CRD) WithCustomColumn(columns ...v1beta1.CustomResourceColumnDefinition) CRD {
	c.Columns = append(c.Columns, columns...)
	return c
}

func (c CRD) WithStatus() CRD {
	c.Status = true
	return c
}

func (c CRD) WithScale() CRD {
	c.Scale = true
	return c
}

func (c CRD) WithCategories(categories ...string) CRD {
	c.Categories = categories
	return c
}

func (c CRD) WithShortNames(shortNames ...string) CRD {
	c.ShortNames = shortNames
	return c
}

func (c CRD) ToCustomResourceDefinition() (apiext.CustomResourceDefinition, error) {
	if c.SchemaObject != nil && c.GVK.Kind == "" {
		t := getType(c.SchemaObject)
		c.GVK.Kind = t.Name()
	}

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
			AdditionalPrinterColumns: c.Columns,
			Group:                    c.GVK.Group,
			Version:                  c.GVK.Version,
			Versions: []apiext.CustomResourceDefinitionVersion{
				{
					Name:    c.GVK.Version,
					Storage: true,
					Served:  true,
				},
			},
			Names: apiext.CustomResourceDefinitionNames{
				Plural:     plural,
				Kind:       c.GVK.Kind,
				Categories: c.Categories,
				ShortNames: c.ShortNames,
			},
		},
	}

	if c.Schema != nil {
		crd.Spec.Validation = &apiext.CustomResourceValidation{
			OpenAPIV3Schema: c.Schema,
		}
	}

	if c.SchemaObject != nil {
		schema, err := openapi.ToOpenAPIFromStruct(c.SchemaObject)
		if err != nil {
			return apiext.CustomResourceDefinition{}, err
		}
		crd.Spec.Validation = &apiext.CustomResourceValidation{
			OpenAPIV3Schema: schema,
		}
	}

	if c.Status {
		crd.Spec.Subresources = &apiext.CustomResourceSubresources{
			Status: &apiext.CustomResourceSubresourceStatus{},
		}
		if c.Scale {
			sel := "Spec.Selector"
			crd.Spec.Subresources.Scale = &apiext.CustomResourceSubresourceScale{
				SpecReplicasPath:   "Spec.Replicas",
				StatusReplicasPath: "Status.Replicas",
				LabelSelectorPath:  &sel,
			}
		}
	}

	if c.NonNamespace {
		crd.Spec.Scope = apiext.ClusterScoped
	} else {
		crd.Spec.Scope = apiext.NamespaceScoped
	}

	return crd, nil
}

func NamespacedType(name string) CRD {
	kindGroup, version := kv.Split(name, "/")
	kind, group := kv.Split(kindGroup, ".")
	kind = convert.Capitalize(kind)
	group = strings.ToLower(group)

	return FromGV(schema.GroupVersion{
		Group:   group,
		Version: version,
	}, kind)
}

func New(group, version string) CRD {
	return CRD{
		GVK: schema.GroupVersionKind{
			Group:   group,
			Version: version,
		},
		PluralName:   "",
		NonNamespace: false,
		Schema:       nil,
		SchemaObject: nil,
		Columns:      nil,
		Status:       false,
		Scale:        false,
		Categories:   nil,
		ShortNames:   nil,
	}
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

func (f *Factory) BatchCreateCRDs(ctx context.Context, crds ...CRD) *Factory {
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		if _, err := f.CreateCRDs(ctx, crds...); err != nil && f.err == nil {
			f.err = err
		}
	}()
	return f
}

func (f *Factory) CreateCRDs(ctx context.Context, crds ...CRD) (map[schema.GroupVersionKind]*apiext.CustomResourceDefinition, error) {
	if len(crds) == 0 {
		return nil, nil
	}

	if ok, err := f.ensureAccess(ctx); err != nil {
		return nil, err
	} else if !ok {
		logrus.Infof("No access to list CRDs, assuming CRDs are pre-created.")
		return nil, err
	}

	crdStatus := map[schema.GroupVersionKind]*apiext.CustomResourceDefinition{}

	ready, err := f.getReadyCRDs(ctx)
	if err != nil {
		return nil, err
	}

	for _, crdDef := range crds {
		crd, err := f.createCRD(ctx, crdDef, ready)
		if err != nil {
			return nil, err
		}
		crdStatus[crdDef.GVK] = crd
	}

	ready, err = f.getReadyCRDs(ctx)
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

		crd, err := f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
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

func (f *Factory) createCRD(ctx context.Context, crdDef CRD, ready map[string]*apiext.CustomResourceDefinition) (*apiext.CustomResourceDefinition, error) {
	plural := crdDef.PluralName
	if plural == "" {
		plural = strings.ToLower(name.GuessPluralName(crdDef.GVK.Kind))
	}

	crd, err := crdDef.ToCustomResourceDefinition()
	if err != nil {
		return nil, err
	}

	existing, ok := ready[crd.Name]
	if ok {
		if !equality.Semantic.DeepEqual(crd.Spec.Subresources, existing.Spec.Subresources) ||
			!equality.Semantic.DeepEqual(crd.Spec.Validation, existing.Spec.Validation) ||
			!equality.Semantic.DeepEqual(crd.Spec.Versions, existing.Spec.Versions) {
			existing.Spec = crd.Spec
			logrus.Infof("Updating CRD %s", crd.Name)
			return f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Update(ctx, existing, metav1.UpdateOptions{})
		}
		return existing, nil
	}

	logrus.Infof("Creating CRD %s", crd.Name)
	if newCrd, err := f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(ctx, &crd, metav1.CreateOptions{}); apierrors.IsAlreadyExists(err) {
		return f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	} else if err != nil {
		return nil, err
	} else {
		return newCrd, nil
	}
}

func (f *Factory) ensureAccess(ctx context.Context) (bool, error) {
	_, err := f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if apierrors.IsForbidden(err) {
		return false, nil
	}
	return true, err
}

func (f *Factory) getReadyCRDs(ctx context.Context) (map[string]*apiext.CustomResourceDefinition, error) {
	list, err := f.CRDClient.ApiextensionsV1beta1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
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
