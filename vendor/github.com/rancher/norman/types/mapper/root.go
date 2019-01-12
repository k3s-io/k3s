package mapper

import (
	"github.com/rancher/norman/types"
)

type Root struct {
	enabled bool
	Mapper  types.Mapper
}

func (m *Root) FromInternal(data map[string]interface{}) {
	if m.enabled {
		m.Mapper.FromInternal(data)
	}
}

func (m *Root) ToInternal(data map[string]interface{}) error {
	if m.enabled {
		return m.Mapper.ToInternal(data)
	}
	return nil
}

func (m *Root) ModifySchema(s *types.Schema, schemas *types.Schemas) error {
	if s.CanList(nil) == nil {
		m.enabled = true
		return m.Mapper.ModifySchema(s, schemas)
	}
	return nil
}
