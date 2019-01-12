package merge

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
)

func APIUpdateMerge(schema *types.Schema, schemas *types.Schemas, dest, src map[string]interface{}, replace bool) map[string]interface{} {
	result := UpdateMerge(schema, schemas, dest, src, replace)
	if s, ok := dest["status"]; ok {
		result["status"] = s
	}
	if m, ok := dest["metadata"]; ok {
		result["metadata"] = mergeMetadata(convert.ToMapInterface(m), convert.ToMapInterface(src["metadata"]))
	}
	return result
}

func UpdateMerge(schema *types.Schema, schemas *types.Schemas, dest, src map[string]interface{}, replace bool) map[string]interface{} {
	return mergeMaps("", nil, schema, schemas, replace, dest, src)
}

func isProtected(k string) bool {
	if !strings.Contains(k, "cattle.io/") || (isField(k) && k != "field.cattle.io/creatorId") {
		return false
	}
	return true
}

func isField(k string) bool {
	return strings.HasPrefix(k, "field.cattle.io/")
}

func mergeProtected(dest, src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return dest
	}

	result := copyMap(dest)

	for k, v := range src {
		if isProtected(k) {
			continue
		}
		result[k] = v
	}

	for k := range dest {
		if isProtected(k) || isField(k) {
			continue
		}
		if _, ok := src[k]; !ok {
			delete(result, k)
		}
	}

	return result
}

func mergeMetadata(dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	result := copyMap(dest)

	labels := convert.ToMapInterface(dest["labels"])
	srcLabels := convert.ToMapInterface(src["labels"])
	labels = mergeProtected(labels, srcLabels)

	annotations := convert.ToMapInterface(dest["annotations"])
	srcAnnotation := convert.ToMapInterface(src["annotations"])
	annotations = mergeProtected(annotations, srcAnnotation)

	result["labels"] = labels
	result["annotations"] = annotations

	return result
}

func merge(field, fieldType string, parentSchema, schema *types.Schema, schemas *types.Schemas, replace bool, dest, src interface{}) interface{} {
	if isMap(field, schema, schemas) {
		return src
	}

	sm, smOk := src.(map[string]interface{})
	dm, dmOk := dest.(map[string]interface{})
	if smOk && dmOk {
		fieldType, fieldSchema := getSchema(field, fieldType, parentSchema, schema, schemas)
		return mergeMaps(fieldType, schema, fieldSchema, schemas, replace, dm, sm)
	}
	return src
}

func getSchema(field, parentFieldType string, parentSchema, schema *types.Schema, schemas *types.Schemas) (string, *types.Schema) {
	if schema == nil {
		if definition.IsMapType(parentFieldType) && parentSchema != nil {
			subType := definition.SubType(parentFieldType)
			s := schemas.Schema(&parentSchema.Version, subType)
			if s != nil && s.InternalSchema != nil {
				s = s.InternalSchema
			}
			return subType, s
		}
		return "", nil
	}
	fieldType := schema.ResourceFields[field].Type
	s := schemas.Schema(&schema.Version, fieldType)
	if s != nil && s.InternalSchema != nil {
		return fieldType, s.InternalSchema
	}
	return fieldType, s
}

func isMap(field string, schema *types.Schema, schemas *types.Schemas) bool {
	if schema == nil {
		return false
	}
	f := schema.ResourceFields[field]
	mapType := definition.IsMapType(f.Type)
	if !mapType {
		return false
	}

	subType := definition.SubType(f.Type)
	return schemas.Schema(&schema.Version, subType) == nil
}

func mergeMaps(fieldType string, parentSchema, schema *types.Schema, schemas *types.Schemas, replace bool, dest map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	result := copyMapReplace(schema, dest, replace)
	for k, v := range src {
		result[k] = merge(k, fieldType, parentSchema, schema, schemas, replace, dest[k], v)
	}
	return result
}

func copyMap(src map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range src {
		result[k] = v
	}
	return result
}

func copyMapReplace(schema *types.Schema, src map[string]interface{}, replace bool) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range src {
		if replace && schema != nil {
			f := schema.ResourceFields[k]
			if f.Update {
				continue
			}
		}
		result[k] = v
	}
	return result
}
