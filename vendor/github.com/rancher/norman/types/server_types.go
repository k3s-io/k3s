package types

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

type ValuesMap struct {
	Foo map[string]interface{}
}

type RawResource struct {
	ID           string                 `json:"id,omitempty" yaml:"id,omitempty"`
	Type         string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Schema       *Schema                `json:"-" yaml:"-"`
	Links        map[string]string      `json:"links,omitempty" yaml:"links,omitempty"`
	Actions      map[string]string      `json:"actions,omitempty" yaml:"actions,omitempty"`
	Values       map[string]interface{} `json:",inline" yaml:",inline"`
	ActionLinks  bool                   `json:"-" yaml:"-"`
	DropReadOnly bool                   `json:"-" yaml:"-"`
}

func (r *RawResource) AddAction(apiContext *APIContext, name string) {
	r.Actions[name] = apiContext.URLBuilder.Action(name, r)
}

func (r *RawResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.ToMap())
}

func (r *RawResource) ToMap() map[string]interface{} {
	data := map[string]interface{}{}
	for k, v := range r.Values {
		data[k] = v
	}

	if r.ID != "" && !r.DropReadOnly {
		data["id"] = r.ID
	}

	if r.Type != "" && !r.DropReadOnly {
		data["type"] = r.Type
	}
	if r.Schema.BaseType != "" && !r.DropReadOnly {
		data["baseType"] = r.Schema.BaseType
	}

	if len(r.Links) > 0 && !r.DropReadOnly {
		data["links"] = r.Links
	}

	if len(r.Actions) > 0 && !r.DropReadOnly {
		if r.ActionLinks {
			data["actionLinks"] = r.Actions
		} else {
			data["actions"] = r.Actions
		}
	}
	return data
}

type ActionHandler func(actionName string, action *Action, request *APIContext) error

type RequestHandler func(request *APIContext, next RequestHandler) error

type QueryFilter func(opts *QueryOptions, schema *Schema, data []map[string]interface{}) []map[string]interface{}

type Validator func(request *APIContext, schema *Schema, data map[string]interface{}) error

type InputFormatter func(request *APIContext, schema *Schema, data map[string]interface{}, create bool) error

type Formatter func(request *APIContext, resource *RawResource)

type CollectionFormatter func(request *APIContext, collection *GenericCollection)

type ErrorHandler func(request *APIContext, err error)

type SubContextAttributeProvider interface {
	Query(apiContext *APIContext, schema *Schema) []*QueryCondition
	Create(apiContext *APIContext, schema *Schema) map[string]interface{}
}

type ResponseWriter interface {
	Write(apiContext *APIContext, code int, obj interface{})
}

type AccessControl interface {
	CanCreate(apiContext *APIContext, schema *Schema) error
	CanList(apiContext *APIContext, schema *Schema) error
	CanGet(apiContext *APIContext, schema *Schema) error
	CanUpdate(apiContext *APIContext, obj map[string]interface{}, schema *Schema) error
	CanDelete(apiContext *APIContext, obj map[string]interface{}, schema *Schema) error
	// CanDo function should not yet be used if a corresponding specific method exists. It has been added to
	// satisfy a specific usecase for the short term until full-blown dynamic RBAC can be implemented.
	CanDo(apiGroup, resource, verb string, apiContext *APIContext, obj map[string]interface{}, schema *Schema) error

	Filter(apiContext *APIContext, schema *Schema, obj map[string]interface{}, context map[string]string) map[string]interface{}
	FilterList(apiContext *APIContext, schema *Schema, obj []map[string]interface{}, context map[string]string) []map[string]interface{}
}

type APIContext struct {
	Action                      string
	ID                          string
	Type                        string
	Link                        string
	Method                      string
	Schema                      *Schema
	Schemas                     *Schemas
	Version                     *APIVersion
	SchemasVersion              *APIVersion
	Query                       url.Values
	ResponseFormat              string
	ReferenceValidator          ReferenceValidator
	ResponseWriter              ResponseWriter
	QueryFilter                 QueryFilter
	SubContextAttributeProvider SubContextAttributeProvider
	URLBuilder                  URLBuilder
	AccessControl               AccessControl
	SubContext                  map[string]string
	Pagination                  *Pagination

	Request  *http.Request
	Response http.ResponseWriter
}

type apiContextKey struct{}

func NewAPIContext(req *http.Request, resp http.ResponseWriter, schemas *Schemas) *APIContext {
	apiCtx := &APIContext{
		Response: resp,
		Schemas:  schemas,
	}
	ctx := context.WithValue(req.Context(), apiContextKey{}, apiCtx)
	apiCtx.Request = req.WithContext(ctx)
	return apiCtx
}

func GetAPIContext(ctx context.Context) *APIContext {
	apiContext, _ := ctx.Value(apiContextKey{}).(*APIContext)
	return apiContext
}

func (r *APIContext) Option(key string) string {
	return r.Query.Get("_" + key)
}

func (r *APIContext) WriteResponse(code int, obj interface{}) {
	r.ResponseWriter.Write(r, code, obj)
}

func (r *APIContext) FilterList(opts *QueryOptions, schema *Schema, obj []map[string]interface{}) []map[string]interface{} {
	return r.QueryFilter(opts, schema, obj)
}

func (r *APIContext) FilterObject(opts *QueryOptions, schema *Schema, obj map[string]interface{}) map[string]interface{} {
	opts.Pagination = nil
	result := r.QueryFilter(opts, schema, []map[string]interface{}{obj})
	if len(result) == 0 {
		return nil
	}
	return result[0]
}

func (r *APIContext) Filter(opts *QueryOptions, schema *Schema, obj interface{}) interface{} {
	switch v := obj.(type) {
	case []map[string]interface{}:
		return r.FilterList(opts, schema, v)
	case map[string]interface{}:
		return r.FilterObject(opts, schema, v)
	}

	return nil
}

var (
	ASC  = SortOrder("asc")
	DESC = SortOrder("desc")
)

type QueryOptions struct {
	Sort       Sort
	Pagination *Pagination
	Conditions []*QueryCondition
	Options    map[string]string
}

type ReferenceValidator interface {
	Validate(resourceType, resourceID string) bool
	Lookup(resourceType, resourceID string) *RawResource
}

type URLBuilder interface {
	Current() string
	Collection(schema *Schema, versionOverride *APIVersion) string
	CollectionAction(schema *Schema, versionOverride *APIVersion, action string) string
	SubContextCollection(subContext *Schema, contextName string, schema *Schema) string
	SchemaLink(schema *Schema) string
	ResourceLink(resource *RawResource) string
	Link(linkName string, resource *RawResource) string
	RelativeToRoot(path string) string
	Version(version APIVersion) string
	Marker(marker string) string
	ReverseSort(order SortOrder) string
	Sort(field string) string
	SetSubContext(subContext string)
	FilterLink(schema *Schema, fieldName string, value string) string
	Action(action string, resource *RawResource) string
	ResourceLinkByID(schema *Schema, id string) string
	ActionLinkByID(schema *Schema, id string, action string) string
}

type StorageContext string

var DefaultStorageContext StorageContext

type Store interface {
	Context() StorageContext
	ByID(apiContext *APIContext, schema *Schema, id string) (map[string]interface{}, error)
	List(apiContext *APIContext, schema *Schema, opt *QueryOptions) ([]map[string]interface{}, error)
	Create(apiContext *APIContext, schema *Schema, data map[string]interface{}) (map[string]interface{}, error)
	Update(apiContext *APIContext, schema *Schema, data map[string]interface{}, id string) (map[string]interface{}, error)
	Delete(apiContext *APIContext, schema *Schema, id string) (map[string]interface{}, error)
	Watch(apiContext *APIContext, schema *Schema, opt *QueryOptions) (chan map[string]interface{}, error)
}
