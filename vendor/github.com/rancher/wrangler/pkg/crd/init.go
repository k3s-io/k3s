package crd

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/kv"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/schemas/openapi"
	"github.com/sirupsen/logrus"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"

	// Ensure the gvks are loaded so that apply works correctly
	_ "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1"
)

const CRDKind = "CustomResourceDefinition"

type Factory struct {
	wg        sync.WaitGroup
	err       error
	CRDClient clientset.Interface
	apply     apply.Apply
}

type CRD struct {
	GVK          schema.GroupVersionKind
	PluralName   string
	SingularName string
	NonNamespace bool
	Schema       *apiextv1.JSONSchemaProps
	SchemaObject interface{}
	Columns      []apiextv1.CustomResourceColumnDefinition
	Status       bool
	Scale        bool
	Categories   []string
	ShortNames   []string
	Labels       map[string]string
	Annotations  map[string]string

	Override runtime.Object
}

func (c CRD) WithSchema(schema *apiextv1.JSONSchemaProps) CRD {
	c.Schema = schema
	return c
}

func (c CRD) WithSchemaFromStruct(obj interface{}) CRD {
	c.SchemaObject = obj
	return c
}

func (c CRD) WithColumn(name, path string) CRD {
	c.Columns = append(c.Columns, apiextv1.CustomResourceColumnDefinition{
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

func tagToColumn(f reflect.StructField) (apiextv1.CustomResourceColumnDefinition, bool) {
	c := apiextv1.CustomResourceColumnDefinition{
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

func readCustomColumns(t reflect.Type, path string) (result []apiextv1.CustomResourceColumnDefinition) {
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

func (c CRD) WithCustomColumn(columns ...apiextv1.CustomResourceColumnDefinition) CRD {
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

func (c CRD) WithGroup(group string) CRD {
	c.GVK.Group = group
	return c
}

func (c CRD) WithShortNames(shortNames ...string) CRD {
	c.ShortNames = shortNames
	return c
}

func (c CRD) ToCustomResourceDefinition() (runtime.Object, error) {
	if c.Override != nil {
		return c.Override, nil
	}

	if c.SchemaObject != nil && c.GVK.Kind == "" {
		t := getType(c.SchemaObject)
		c.GVK.Kind = t.Name()
	}

	if c.SchemaObject != nil && c.GVK.Version == "" {
		t := getType(c.SchemaObject)
		c.GVK.Version = filepath.Base(t.PkgPath())
	}

	if c.SchemaObject != nil && c.GVK.Group == "" {
		t := getType(c.SchemaObject)
		c.GVK.Group = filepath.Base(filepath.Dir(t.PkgPath()))
	}

	plural := c.PluralName
	if plural == "" {
		plural = strings.ToLower(name.GuessPluralName(c.GVK.Kind))
	}

	singular := c.SingularName
	if singular == "" {
		singular = strings.ToLower(c.GVK.Kind)
	}

	name := strings.ToLower(plural + "." + c.GVK.Group)

	crd := apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: c.GVK.Group,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:                     c.GVK.Version,
					Storage:                  true,
					Served:                   true,
					AdditionalPrinterColumns: c.Columns,
				},
			},
			Names: apiextv1.CustomResourceDefinitionNames{
				Plural:     plural,
				Singular:   singular,
				Kind:       c.GVK.Kind,
				Categories: c.Categories,
				ShortNames: c.ShortNames,
			},
			PreserveUnknownFields: false,
		},
	}

	if c.Schema != nil {
		crd.Spec.Versions[0].Schema = &apiextv1.CustomResourceValidation{
			OpenAPIV3Schema: c.Schema,
		}
	}

	if c.SchemaObject != nil {
		schema, err := openapi.ToOpenAPIFromStruct(c.SchemaObject)
		if err != nil {
			return nil, err
		}
		crd.Spec.Versions[0].Schema = &apiextv1.CustomResourceValidation{
			OpenAPIV3Schema: schema,
		}
	}

	// add a dummy schema because v1 requires OpenAPIV3Schema to be set
	if crd.Spec.Versions[0].Schema == nil {
		crd.Spec.Versions[0].Schema = &apiextv1.CustomResourceValidation{
			OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
				Type: "object",
				Properties: map[string]apiextv1.JSONSchemaProps{
					"spec": {
						XPreserveUnknownFields: &[]bool{true}[0],
					},
					"status": {
						XPreserveUnknownFields: &[]bool{true}[0],
					},
				},
			},
		}
	}

	if c.Status {
		crd.Spec.Versions[0].Subresources = &apiextv1.CustomResourceSubresources{
			Status: &apiextv1.CustomResourceSubresourceStatus{},
		}
		if c.Scale {
			sel := "Spec.Selector"
			crd.Spec.Versions[0].Subresources.Scale = &apiextv1.CustomResourceSubresourceScale{
				SpecReplicasPath:   "Spec.Replicas",
				StatusReplicasPath: "Status.Replicas",
				LabelSelectorPath:  &sel,
			}
		}
	}

	if c.NonNamespace {
		crd.Spec.Scope = apiextv1.ClusterScoped
	} else {
		crd.Spec.Scope = apiextv1.NamespaceScoped
	}

	crd.Labels = c.Labels
	crd.Annotations = c.Annotations

	// Convert to unstructured to ensure that PreserveUnknownFields=false is set because the struct will omit false
	mapData, err := convert.EncodeToMap(crd)
	if err != nil {
		return nil, err
	}
	mapData["kind"] = CRDKind
	mapData["apiVersion"] = apiextv1.SchemeGroupVersion.String()

	return &unstructured.Unstructured{
		Object: mapData,
	}, unstructured.SetNestedField(mapData, false, "spec", "preserveUnknownFields")
}

func (c CRD) ToCustomResourceDefinitionV1Beta1() (*apiextv1beta1.CustomResourceDefinition, error) {
	toConvertCRD, err := c.ToCustomResourceDefinition()
	if err != nil {
		return nil, err
	}
	if toConvertCRD == nil {
		return nil, fmt.Errorf("cannot convert empty CRD runtime object to apiextensions v1beta1 CRD object")
	}

	unstructuredCRD, ok := toConvertCRD.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("could not convert CRD runtime object to *unstructured.Unstructured")
	}
	var v1CRD *apiextv1.CustomResourceDefinition
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredCRD.UnstructuredContent(), &v1CRD); err != nil {
		return nil, err
	}

	internalCRD := &apiext.CustomResourceDefinition{}
	if err := apiextv1.Convert_v1_CustomResourceDefinition_To_apiextensions_CustomResourceDefinition(v1CRD, internalCRD, nil); err != nil {
		return nil, err
	}
	v1beta1CRD := &apiextv1beta1.CustomResourceDefinition{}
	if err := apiextv1beta1.Convert_apiextensions_CustomResourceDefinition_To_v1beta1_CustomResourceDefinition(internalCRD, v1beta1CRD, nil); err != nil {
		return nil, err
	}

	// GVK is dropped during conversion, so we must add it.
	v1beta1CRD.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Group:   apiextv1beta1.SchemeGroupVersion.Group,
		Version: apiextv1beta1.SchemeGroupVersion.Version,
		Kind:    CRDKind,
	})
	return v1beta1CRD, nil
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

func NewFactoryFromClient(config *rest.Config) (*Factory, error) {
	apply, err := apply.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	f, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Factory{
		CRDClient: f,
		apply:     apply.WithDynamicLookup().WithNoDelete(),
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

func (f *Factory) CreateCRDs(ctx context.Context, crds ...CRD) (map[schema.GroupVersionKind]*apiextv1.CustomResourceDefinition, error) {
	if len(crds) == 0 {
		return nil, nil
	}

	if ok, err := f.ensureAccess(ctx); err != nil {
		return nil, err
	} else if !ok {
		logrus.Infof("No access to list CRDs, assuming CRDs are pre-created.")
		return nil, err
	}

	crdStatus := map[schema.GroupVersionKind]*apiextv1.CustomResourceDefinition{}

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

func (f *Factory) waitCRD(ctx context.Context, crdName string, gvk schema.GroupVersionKind, crdStatus map[schema.GroupVersionKind]*apiextv1.CustomResourceDefinition) error {
	logrus.Infof("Waiting for CRD %s to become available", crdName)
	defer logrus.Infof("Done waiting for CRD %s to become available", crdName)

	first := true
	return wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		if !first {
			logrus.Infof("Waiting for CRD %s to become available", crdName)
		}
		first = false

		crd, err := f.CRDClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextv1.Established:
				if cond.Status == apiextv1.ConditionTrue {
					crdStatus[gvk] = crd
					return true, err
				}
			case apiextv1.NamesAccepted:
				if cond.Status == apiextv1.ConditionFalse {
					logrus.Infof("Name conflict on %s: %v\n", crdName, cond.Reason)
				}
			}
		}

		return false, ctx.Err()
	})
}

func (f *Factory) createCRD(ctx context.Context, crdDef CRD, ready map[string]*apiextv1.CustomResourceDefinition) (*apiextv1.CustomResourceDefinition, error) {
	crd, err := crdDef.ToCustomResourceDefinition()
	if err != nil {
		return nil, err
	}

	meta, err := meta.Accessor(crd)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Applying CRD %s", meta.GetName())
	if err := f.apply.WithOwner(crd).ApplyObjects(crd); err != nil {
		return nil, err
	}

	return f.CRDClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, meta.GetName(), metav1.GetOptions{})
}

func (f *Factory) ensureAccess(ctx context.Context) (bool, error) {
	_, err := f.CRDClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if apierrors.IsForbidden(err) {
		return false, nil
	}
	return true, err
}

func (f *Factory) getReadyCRDs(ctx context.Context) (map[string]*apiextv1.CustomResourceDefinition, error) {
	list, err := f.CRDClient.ApiextensionsV1().CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := map[string]*apiextv1.CustomResourceDefinition{}

	for i, crd := range list.Items {
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextv1.Established:
				if cond.Status == apiextv1.ConditionTrue {
					result[crd.Name] = &list.Items[i]
				}
			}
		}
	}

	return result, nil
}
