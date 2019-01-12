package types

import (
	"fmt"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"github.com/rancher/norman/types/values"
)

type Mapper interface {
	FromInternal(data map[string]interface{})
	ToInternal(data map[string]interface{}) error
	ModifySchema(schema *Schema, schemas *Schemas) error
}

type Mappers []Mapper

func (m Mappers) FromInternal(data map[string]interface{}) {
	for _, mapper := range m {
		mapper.FromInternal(data)
	}
}

func (m Mappers) ToInternal(data map[string]interface{}) error {
	var errors []error
	for i := len(m) - 1; i >= 0; i-- {
		errors = append(errors, m[i].ToInternal(data))
	}
	return NewErrors(errors...)
}

func (m Mappers) ModifySchema(schema *Schema, schemas *Schemas) error {
	for _, mapper := range m {
		if err := mapper.ModifySchema(schema, schemas); err != nil {
			return err
		}
	}
	return nil
}

type typeMapper struct {
	Mappers         []Mapper
	root            bool
	typeName        string
	subSchemas      map[string]*Schema
	subArraySchemas map[string]*Schema
	subMapSchemas   map[string]*Schema
}

func (t *typeMapper) FromInternal(data map[string]interface{}) {
	name, _ := values.GetValueN(data, "metadata", "name").(string)
	namespace, _ := values.GetValueN(data, "metadata", "namespace").(string)

	for fieldName, schema := range t.subSchemas {
		if schema.Mapper == nil {
			continue
		}
		fieldData, _ := data[fieldName].(map[string]interface{})
		schema.Mapper.FromInternal(fieldData)
	}

	for fieldName, schema := range t.subMapSchemas {
		if schema.Mapper == nil {
			continue
		}
		datas, _ := data[fieldName].(map[string]interface{})
		for _, fieldData := range datas {
			mapFieldData, _ := fieldData.(map[string]interface{})
			schema.Mapper.FromInternal(mapFieldData)
		}
	}

	for fieldName, schema := range t.subArraySchemas {
		if schema.Mapper == nil {
			continue
		}
		datas, _ := data[fieldName].([]interface{})
		for _, fieldData := range datas {
			mapFieldData, _ := fieldData.(map[string]interface{})
			schema.Mapper.FromInternal(mapFieldData)
		}
	}

	if _, ok := data["type"]; !ok && data != nil {
		data["type"] = t.typeName
	}

	Mappers(t.Mappers).FromInternal(data)

	if data != nil && t.root {
		if _, ok := data["id"]; ok {
			if namespace != "" {
				id, _ := data["id"].(string)
				data["id"] = namespace + ":" + id
			}
		} else {
			if name != "" {
				if namespace == "" {
					data["id"] = name
				} else {
					data["id"] = namespace + ":" + name
				}
			}
		}
	}

	if _, ok := data["type"]; !ok && data != nil {
		if _, ok := data["id"]; ok {
			data["type"] = t.typeName
		}
	}
}

func (t *typeMapper) ToInternal(data map[string]interface{}) error {
	errors := Errors{}
	errors.Add(Mappers(t.Mappers).ToInternal(data))

	for fieldName, schema := range t.subArraySchemas {
		if schema.Mapper == nil {
			continue
		}
		datas, _ := data[fieldName].([]interface{})
		for _, fieldData := range datas {
			errors.Add(schema.Mapper.ToInternal(convert.ToMapInterface(fieldData)))
		}
	}

	for fieldName, schema := range t.subMapSchemas {
		if schema.Mapper == nil {
			continue
		}
		datas, _ := data[fieldName].(map[string]interface{})
		for _, fieldData := range datas {
			errors.Add(schema.Mapper.ToInternal(convert.ToMapInterface(fieldData)))
		}
	}

	for fieldName, schema := range t.subSchemas {
		if schema.Mapper == nil {
			continue
		}
		fieldData, _ := data[fieldName].(map[string]interface{})
		errors.Add(schema.Mapper.ToInternal(fieldData))
	}

	return errors.Err()
}

func (t *typeMapper) ModifySchema(schema *Schema, schemas *Schemas) error {
	t.subSchemas = map[string]*Schema{}
	t.subArraySchemas = map[string]*Schema{}
	t.subMapSchemas = map[string]*Schema{}
	t.typeName = fmt.Sprintf("%s/schemas/%s", schema.Version.Path, schema.ID)

	mapperSchema := schema
	if schema.InternalSchema != nil {
		mapperSchema = schema.InternalSchema
	}
	for name, field := range mapperSchema.ResourceFields {
		fieldType := field.Type
		targetMap := t.subSchemas
		if definition.IsArrayType(fieldType) {
			fieldType = definition.SubType(fieldType)
			targetMap = t.subArraySchemas
		} else if definition.IsMapType(fieldType) {
			fieldType = definition.SubType(fieldType)
			targetMap = t.subMapSchemas
		}

		schema := schemas.Schema(&schema.Version, fieldType)
		if schema != nil {
			targetMap[name] = schema
		}
	}

	return Mappers(t.Mappers).ModifySchema(schema, schemas)
}
