package mapper

import (
	"github.com/rancher/norman/types"
)

type ReadOnly struct {
	Field     string
	Optional  bool
	SubFields bool
}

func (r ReadOnly) FromInternal(data map[string]interface{}) {
}

func (r ReadOnly) ToInternal(data map[string]interface{}) error {
	return nil
}

func (r ReadOnly) readOnly(field types.Field, schema *types.Schema, schemas *types.Schemas) types.Field {
	field.Create = false
	field.Update = false

	if r.SubFields {
		subSchema := schemas.Schema(&schema.Version, field.Type)
		if subSchema != nil {
			for name, field := range subSchema.ResourceFields {
				field.Create = false
				field.Update = false
				subSchema.ResourceFields[name] = field
			}
		}
	}

	return field
}

func (r ReadOnly) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if r.Field == "*" {
		for name, field := range schema.ResourceFields {
			schema.ResourceFields[name] = r.readOnly(field, schema, schemas)
		}
		return nil
	}

	if err := ValidateField(r.Field, schema); err != nil {
		if r.Optional {
			return nil
		}
		return err
	}

	field := schema.ResourceFields[r.Field]
	schema.ResourceFields[r.Field] = r.readOnly(field, schema, schemas)

	return nil
}
