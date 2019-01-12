package mapper

import (
	"github.com/rancher/norman/types"
)

type Scope struct {
	If      types.TypeScope
	IfNot   types.TypeScope
	Mappers []types.Mapper
	run     bool
}

func (s *Scope) FromInternal(data map[string]interface{}) {
	if s.run {
		types.Mappers(s.Mappers).FromInternal(data)
	}
}

func (s *Scope) ToInternal(data map[string]interface{}) error {
	if s.run {
		return types.Mappers(s.Mappers).ToInternal(data)
	}
	return nil
}

func (s *Scope) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if s.If != "" {
		s.run = schema.Scope == s.If
	}
	if s.IfNot != "" {
		s.run = schema.Scope != s.IfNot
	}
	if s.run {
		return types.Mappers(s.Mappers).ModifySchema(schema, schemas)
	}
	return nil
}
