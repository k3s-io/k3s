package mapper

import (
	"strings"

	"encoding/json"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type JSONEncode struct {
	Field            string
	IgnoreDefinition bool
	Separator        string
}

func (m JSONEncode) FromInternal(data map[string]interface{}) {
	if v, ok := values.RemoveValue(data, strings.Split(m.Field, m.getSep())...); ok {
		obj := map[string]interface{}{}
		if err := json.Unmarshal([]byte(convert.ToString(v)), &obj); err == nil {
			values.PutValue(data, obj, strings.Split(m.Field, m.getSep())...)
		} else {
			log.Errorf("Failed to unmarshal json field: %v", err)
		}
	}
}

func (m JSONEncode) ToInternal(data map[string]interface{}) error {
	if v, ok := values.RemoveValue(data, strings.Split(m.Field, m.getSep())...); ok && v != nil {
		if bytes, err := json.Marshal(v); err == nil {
			values.PutValue(data, string(bytes), strings.Split(m.Field, m.getSep())...)
		}
	}
	return nil
}

func (m JSONEncode) getSep() string {
	if m.Separator == "" {
		return "/"
	}
	return m.Separator
}

func (m JSONEncode) ModifySchema(s *types.Schema, schemas *types.Schemas) error {
	if m.IgnoreDefinition {
		return nil
	}

	return ValidateField(m.Field, s)
}
