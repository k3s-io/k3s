package parse

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type DefaultSubContextAttributeProvider struct {
}

func (d *DefaultSubContextAttributeProvider) Query(apiContext *types.APIContext, schema *types.Schema) []*types.QueryCondition {
	var result []*types.QueryCondition

	for name, value := range d.create(apiContext, schema) {
		result = append(result, types.NewConditionFromString(name, types.ModifierEQ, value))
	}

	return result
}

func (d *DefaultSubContextAttributeProvider) Create(apiContext *types.APIContext, schema *types.Schema) map[string]interface{} {
	result := map[string]interface{}{}
	for key, value := range d.create(apiContext, schema) {
		result[key] = value
	}
	return result
}

func (d *DefaultSubContextAttributeProvider) create(apiContext *types.APIContext, schema *types.Schema) map[string]string {
	result := map[string]string{}

	for subContextSchemaID, value := range apiContext.SubContext {
		subContextSchema := apiContext.Schemas.Schema(nil, subContextSchemaID)
		if subContextSchema == nil {
			continue
		}

		ref := convert.ToReference(subContextSchema.ID)
		fullRef := convert.ToFullReference(subContextSchema.Version.Path, subContextSchema.ID)

		for name, field := range schema.ResourceFields {
			if field.Type == ref || field.Type == fullRef {
				result[name] = value
				break
			}
		}
	}

	return result
}
