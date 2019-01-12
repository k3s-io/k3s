package schema

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/empty"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/slice"
)

type Store struct {
	empty.Store
}

func NewSchemaStore() types.Store {
	return &Store{}
}

func (s *Store) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	for _, schema := range apiContext.Schemas.SchemasForVersion(*apiContext.Version) {
		if strings.EqualFold(schema.ID, id) {
			schemaData := map[string]interface{}{}

			data, err := json.Marshal(s.modifyForAccessControl(apiContext, *schema))
			if err != nil {
				return nil, err
			}

			return schemaData, json.Unmarshal(data, &schemaData)
		}
	}
	return nil, httperror.NewAPIError(httperror.NotFound, "no such schema")
}

func (s *Store) modifyForAccessControl(context *types.APIContext, schema types.Schema) *types.Schema {
	var resourceMethods []string
	if slice.ContainsString(schema.ResourceMethods, http.MethodPut) && schema.CanUpdate(context) == nil {
		resourceMethods = append(resourceMethods, http.MethodPut)
	}
	if slice.ContainsString(schema.ResourceMethods, http.MethodDelete) && schema.CanDelete(context) == nil {
		resourceMethods = append(resourceMethods, http.MethodDelete)
	}
	if slice.ContainsString(schema.ResourceMethods, http.MethodGet) && schema.CanGet(context) == nil {
		resourceMethods = append(resourceMethods, http.MethodGet)
	}

	var collectionMethods []string
	if slice.ContainsString(schema.CollectionMethods, http.MethodPost) && schema.CanCreate(context) == nil {
		collectionMethods = append(collectionMethods, http.MethodPost)
	}
	if slice.ContainsString(schema.CollectionMethods, http.MethodGet) && schema.CanList(context) == nil {
		collectionMethods = append(collectionMethods, http.MethodGet)
	}

	schema.ResourceMethods = resourceMethods
	schema.CollectionMethods = collectionMethods

	return &schema
}

func (s *Store) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return nil, nil
}

func (s *Store) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	schemaMap := apiContext.Schemas.SchemasForVersion(*apiContext.Version)
	schemas := make([]*types.Schema, 0, len(schemaMap))
	schemaData := make([]map[string]interface{}, 0, len(schemaMap))

	included := map[string]bool{}

	for _, schema := range schemaMap {
		if included[schema.ID] {
			continue
		}

		if schema.CanList(apiContext) == nil || schema.CanGet(apiContext) == nil {
			schemas = s.addSchema(apiContext, schema, schemaMap, schemas, included)
		}
	}

	data, err := json.Marshal(schemas)
	if err != nil {
		return nil, err
	}

	return schemaData, json.Unmarshal(data, &schemaData)
}

func (s *Store) addSchema(apiContext *types.APIContext, schema *types.Schema, schemaMap map[string]*types.Schema, schemas []*types.Schema, included map[string]bool) []*types.Schema {
	included[schema.ID] = true
	schemas = s.traverseAndAdd(apiContext, schema, schemaMap, schemas, included)
	schemas = append(schemas, s.modifyForAccessControl(apiContext, *schema))
	return schemas
}

func (s *Store) traverseAndAdd(apiContext *types.APIContext, schema *types.Schema, schemaMap map[string]*types.Schema, schemas []*types.Schema, included map[string]bool) []*types.Schema {
	for _, field := range schema.ResourceFields {
		t := ""
		subType := field.Type
		for subType != t {
			t = subType
			subType = definition.SubType(t)
		}

		if refSchema, ok := schemaMap[t]; ok && !included[t] {
			schemas = s.addSchema(apiContext, refSchema, schemaMap, schemas, included)
		}
	}

	for _, action := range schema.ResourceActions {
		for _, t := range []string{action.Output, action.Input} {
			if t == "" {
				continue
			}

			if refSchema, ok := schemaMap[t]; ok && !included[t] {
				schemas = s.addSchema(apiContext, refSchema, schemaMap, schemas, included)
			}
		}
	}

	for _, action := range schema.CollectionActions {
		for _, t := range []string{action.Output, action.Input} {
			if t == "" {
				continue
			}

			if refSchema, ok := schemaMap[t]; ok && !included[t] {
				schemas = s.addSchema(apiContext, refSchema, schemaMap, schemas, included)
			}
		}
	}

	return schemas
}
