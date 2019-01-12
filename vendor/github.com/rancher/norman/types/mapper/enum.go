package mapper

import (
	"github.com/rancher/norman/types"
)

type Enum struct {
	Field   string
	Options []string
}

func (e Enum) FromInternal(data map[string]interface{}) {
}

func (e Enum) ToInternal(data map[string]interface{}) error {
	return nil
}

func (e Enum) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if err := ValidateField(e.Field, schema); err != nil {
		return err
	}

	f := schema.ResourceFields[e.Field]
	f.Type = "enum"
	f.Options = e.Options
	schema.ResourceFields[e.Field] = f
	return nil
}
