package builtin

import (
	"net/http"

	"github.com/rancher/norman/store/schema"
	"github.com/rancher/norman/types"
)

var (
	Version = types.APIVersion{
		Group:   "meta.cattle.io",
		Version: "v1",
		Path:    "/meta",
	}

	Schema = types.Schema{
		ID:                "schema",
		PluralName:        "schemas",
		Version:           Version,
		CollectionMethods: []string{"GET"},
		ResourceMethods:   []string{"GET"},
		ResourceFields: map[string]types.Field{
			"collectionActions": {Type: "map[json]"},
			"collectionFields":  {Type: "map[json]"},
			"collectionFilters": {Type: "map[json]"},
			"collectionMethods": {Type: "array[string]"},
			"pluralName":        {Type: "string"},
			"resourceActions":   {Type: "map[json]"},
			"resourceFields":    {Type: "map[json]"},
			"resourceMethods":   {Type: "array[string]"},
			"version":           {Type: "map[json]"},
		},
		Formatter: SchemaFormatter,
		Store:     schema.NewSchemaStore(),
	}

	Error = types.Schema{
		ID:                "error",
		Version:           Version,
		ResourceMethods:   []string{},
		CollectionMethods: []string{},
		ResourceFields: map[string]types.Field{
			"code":      {Type: "string"},
			"detail":    {Type: "string", Nullable: true},
			"message":   {Type: "string", Nullable: true},
			"fieldName": {Type: "string", Nullable: true},
			"status":    {Type: "int"},
		},
	}

	Collection = types.Schema{
		ID:                "collection",
		Version:           Version,
		ResourceMethods:   []string{},
		CollectionMethods: []string{},
		ResourceFields: map[string]types.Field{
			"data":       {Type: "array[json]"},
			"pagination": {Type: "map[json]"},
			"sort":       {Type: "map[json]"},
			"filters":    {Type: "map[json]"},
		},
	}

	APIRoot = types.Schema{
		ID:                "apiRoot",
		Version:           Version,
		CollectionMethods: []string{"GET"},
		ResourceMethods:   []string{"GET"},
		ResourceFields: map[string]types.Field{
			"apiVersion": {Type: "map[json]"},
			"path":       {Type: "string"},
		},
		Formatter: APIRootFormatter,
		Store:     NewAPIRootStore(nil),
	}

	Schemas = types.NewSchemas().
		AddSchema(Schema).
		AddSchema(Error).
		AddSchema(Collection).
		AddSchema(APIRoot)
)

func apiVersionFromMap(schemas *types.Schemas, apiVersion map[string]interface{}) types.APIVersion {
	path, _ := apiVersion["path"].(string)
	version, _ := apiVersion["version"].(string)
	group, _ := apiVersion["group"].(string)

	apiVersionObj := types.APIVersion{
		Path:    path,
		Version: version,
		Group:   group,
	}

	for _, testVersion := range schemas.Versions() {
		if testVersion.Equals(&apiVersionObj) {
			return testVersion
		}
	}

	return apiVersionObj
}

func SchemaFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	data, _ := resource.Values["version"].(map[string]interface{})
	apiVersion := apiVersionFromMap(apiContext.Schemas, data)

	schema := apiContext.Schemas.Schema(&apiVersion, resource.ID)
	if schema == nil {
		return
	}

	collectionLink := getSchemaCollectionLink(apiContext, schema, &apiVersion)
	if collectionLink != "" {
		resource.Links["collection"] = collectionLink
	}

	resource.Links["self"] = apiContext.URLBuilder.SchemaLink(schema)
}

func getSchemaCollectionLink(apiContext *types.APIContext, schema *types.Schema, apiVersion *types.APIVersion) string {
	if schema != nil && contains(schema.CollectionMethods, http.MethodGet) {
		return apiContext.URLBuilder.Collection(schema, apiVersion)
	}
	return ""
}

func contains(list []string, needle string) bool {
	for _, v := range list {
		if v == needle {
			return true
		}
	}
	return false
}
