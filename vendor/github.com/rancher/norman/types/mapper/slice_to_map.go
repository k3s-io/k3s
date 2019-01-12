package mapper

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/definition"
)

type SliceToMap struct {
	Field string
	Key   string
}

func (s SliceToMap) FromInternal(data map[string]interface{}) {
	datas, _ := data[s.Field].([]interface{})
	result := map[string]interface{}{}

	for _, item := range datas {
		if mapItem, ok := item.(map[string]interface{}); ok {
			name, _ := mapItem[s.Key].(string)
			delete(mapItem, s.Key)
			result[name] = mapItem
		}
	}

	if len(result) > 0 {
		data[s.Field] = result
	}
}

func (s SliceToMap) ToInternal(data map[string]interface{}) error {
	datas, _ := data[s.Field].(map[string]interface{})
	var result []interface{}

	for name, item := range datas {
		mapItem, _ := item.(map[string]interface{})
		if mapItem != nil {
			mapItem[s.Key] = name
			result = append(result, mapItem)
		}
	}

	if len(result) > 0 {
		data[s.Field] = result
	} else if datas != nil {
		data[s.Field] = result
	}

	return nil
}

func (s SliceToMap) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	err := ValidateField(s.Field, schema)
	if err != nil {
		return err
	}

	subSchema, subFieldName, _, _, err := getField(schema, schemas, fmt.Sprintf("%s/%s", s.Field, s.Key))
	if err != nil {
		return err
	}

	field := schema.ResourceFields[s.Field]
	if !definition.IsArrayType(field.Type) {
		return fmt.Errorf("field %s on %s is not an array", s.Field, schema.ID)
	}

	field.Type = "map[" + definition.SubType(field.Type) + "]"
	schema.ResourceFields[s.Field] = field

	delete(subSchema.ResourceFields, subFieldName)

	return nil
}
