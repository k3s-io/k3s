package mapper

import (
	"github.com/rancher/norman/types"
)

type ChangeType struct {
	Field string
	Type  string
}

func (c ChangeType) FromInternal(data map[string]interface{}) {
}

func (c ChangeType) ToInternal(data map[string]interface{}) error {
	return nil
}

func (c ChangeType) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if err := ValidateField(c.Field, schema); err != nil {
		return err
	}

	f := schema.ResourceFields[c.Field]
	f.Type = c.Type
	schema.ResourceFields[c.Field] = f
	return nil
}
