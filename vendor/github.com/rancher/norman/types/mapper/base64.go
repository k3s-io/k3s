package mapper

import (
	"encoding/base64"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

type Base64 struct {
	Field            string
	IgnoreDefinition bool
	Separator        string
}

func (m Base64) FromInternal(data map[string]interface{}) {
	if v, ok := values.RemoveValue(data, strings.Split(m.Field, m.getSep())...); ok {
		str := convert.ToString(v)
		if str == "" {
			return
		}

		newData, err := base64.StdEncoding.DecodeString(str)
		if err != nil {
			log.Errorf("failed to base64 decode data")
		}

		values.PutValue(data, string(newData), strings.Split(m.Field, m.getSep())...)
	}
}

func (m Base64) ToInternal(data map[string]interface{}) error {
	if v, ok := values.RemoveValue(data, strings.Split(m.Field, m.getSep())...); ok {
		str := convert.ToString(v)
		if str == "" {
			return nil
		}

		newData := base64.StdEncoding.EncodeToString([]byte(str))
		values.PutValue(data, newData, strings.Split(m.Field, m.getSep())...)
	}

	return nil
}

func (m Base64) ModifySchema(s *types.Schema, schemas *types.Schemas) error {
	if !m.IgnoreDefinition {
		if err := ValidateField(m.Field, s); err != nil {
			return err
		}
	}

	return nil
}

func (m Base64) getSep() string {
	if m.Separator == "" {
		return "/"
	}
	return m.Separator
}
