package mapper

import (
	"strings"

	"github.com/rancher/norman/types"
)

type Access struct {
	Fields   map[string]string
	Optional bool
}

func (e Access) FromInternal(data map[string]interface{}) {
}

func (e Access) ToInternal(data map[string]interface{}) error {
	return nil
}

func (e Access) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	for name, access := range e.Fields {
		if err := ValidateField(name, schema); err != nil {
			if e.Optional {
				continue
			}
			return err
		}

		field := schema.ResourceFields[name]
		field.Create = strings.Contains(access, "c")
		field.Update = strings.Contains(access, "u")
		field.WriteOnly = strings.Contains(access, "o")

		schema.ResourceFields[name] = field
	}
	return nil
}
