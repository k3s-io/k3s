package mapper

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

type LabelField struct {
	Field string
}

func (e LabelField) FromInternal(data map[string]interface{}) {
	v, ok := values.RemoveValue(data, "labels", "field.cattle.io/"+e.Field)
	if ok {
		data[e.Field] = v
	}
}

func (e LabelField) ToInternal(data map[string]interface{}) error {
	v, ok := data[e.Field]
	if ok {
		values.PutValue(data, v, "labels", "field.cattle.io/"+e.Field)
	}
	return nil
}

func (e LabelField) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return ValidateField(e.Field, schema)
}
