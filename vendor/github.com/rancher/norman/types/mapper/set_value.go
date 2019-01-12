package mapper

import (
	"fmt"

	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
)

type SetValue struct {
	Field, To        string
	Value            interface{}
	IfEq             interface{}
	IgnoreDefinition bool
}

func (s SetValue) FromInternal(data map[string]interface{}) {
	if s.IfEq == nil {
		values.PutValue(data, s.Value, strings.Split(s.getTo(), "/")...)
		return
	}

	v, ok := values.GetValue(data, strings.Split(s.Field, "/")...)
	if !ok {
		return
	}

	if v == s.IfEq {
		values.PutValue(data, s.Value, strings.Split(s.getTo(), "/")...)
	}
}

func (s SetValue) getTo() string {
	if s.To == "" {
		return s.Field
	}
	return s.To
}

func (s SetValue) ToInternal(data map[string]interface{}) error {
	v, ok := values.GetValue(data, strings.Split(s.getTo(), "/")...)
	if !ok {
		return nil
	}

	if s.IfEq == nil {
		values.RemoveValue(data, strings.Split(s.Field, "/")...)
	} else if v == s.Value {
		values.PutValue(data, s.IfEq, strings.Split(s.Field, "/")...)
	}

	return nil
}

func (s SetValue) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	if s.IgnoreDefinition {
		return nil
	}

	_, _, _, ok, err := getField(schema, schemas, s.getTo())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("failed to find defined field for %s on schemas %s", s.getTo(), schema.ID)
	}

	return nil
}
