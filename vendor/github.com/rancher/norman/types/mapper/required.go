package mapper

import (
	"github.com/rancher/norman/types"
)

type Required struct {
	Fields []string
}

func (e Required) FromInternal(data map[string]interface{}) {
}

func (e Required) ToInternal(data map[string]interface{}) error {
	return nil
}

func (e Required) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	for _, field := range e.Fields {
		if err := ValidateField(field, schema); err != nil {
			return err
		}

		f := schema.ResourceFields[field]
		f.Required = true
		schema.ResourceFields[field] = f
	}

	return nil
}
