package mapper

import (
	"github.com/rancher/norman/types"
)

type Condition struct {
	Field  string
	Value  interface{}
	Mapper types.Mapper
}

func (m Condition) FromInternal(data map[string]interface{}) {
	if data[m.Field] == m.Value {
		m.Mapper.FromInternal(data)
	}
}

func (m Condition) ToInternal(data map[string]interface{}) error {
	if data[m.Field] == m.Value {
		return m.Mapper.ToInternal(data)
	}
	return nil
}

func (m Condition) ModifySchema(s *types.Schema, schemas *types.Schemas) error {
	return m.Mapper.ModifySchema(s, schemas)
}
