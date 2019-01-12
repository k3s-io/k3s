package mapper

import (
	"github.com/rancher/norman/types"
)

var displayNameMappers = types.Mappers{
	&Move{From: "name", To: "id"},
	&Move{From: "displayName", To: "name"},
	Access{Fields: map[string]string{
		"id":   "",
		"name": "cru",
	}},
}

type DisplayName struct {
}

func (d DisplayName) FromInternal(data map[string]interface{}) {
	displayNameMappers.FromInternal(data)
}

func (d DisplayName) ToInternal(data map[string]interface{}) error {
	return displayNameMappers.ToInternal(data)
}

func (d DisplayName) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return displayNameMappers.ModifySchema(schema, schemas)
}
