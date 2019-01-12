package types

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/rancher/norman/name"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
)

type SchemaCollection struct {
	Data []Schema
}

type SchemasInitFunc func(*Schemas) *Schemas

type SchemaHook func(*Schema)

type MappersFactory func() []Mapper

type BackReference struct {
	FieldName string
	Schema    *Schema
}

type Schemas struct {
	sync.Mutex
	processingTypes    map[reflect.Type]*Schema
	typeNames          map[reflect.Type]string
	schemasByPath      map[string]map[string]*Schema
	mappers            map[string]map[string][]Mapper
	references         map[string][]BackReference
	embedded           map[string]*Schema
	DefaultMappers     MappersFactory
	DefaultPostMappers MappersFactory
	versions           []APIVersion
	schemas            []*Schema
	AddHook            SchemaHook
	errors             []error
}

func NewSchemas() *Schemas {
	return &Schemas{
		processingTypes: map[reflect.Type]*Schema{},
		typeNames:       map[reflect.Type]string{},
		schemasByPath:   map[string]map[string]*Schema{},
		mappers:         map[string]map[string][]Mapper{},
		references:      map[string][]BackReference{},
		embedded:        map[string]*Schema{},
	}
}

func (s *Schemas) Init(initFunc SchemasInitFunc) *Schemas {
	return initFunc(s)
}

func (s *Schemas) Err() error {
	return NewErrors(s.errors...)
}

func (s *Schemas) AddSchemas(schema *Schemas) *Schemas {
	for _, schema := range schema.Schemas() {
		s.AddSchema(*schema)
	}
	return s
}

func (s *Schemas) RemoveSchema(schema Schema) *Schemas {
	s.Lock()
	defer s.Unlock()
	return s.doRemoveSchema(schema)
}

func (s *Schemas) doRemoveSchema(schema Schema) *Schemas {
	delete(s.schemasByPath[schema.Version.Path], schema.ID)

	s.removeReferences(&schema)

	if schema.Embed {
		s.removeEmbed(&schema)
	}

	return s
}

func (s *Schemas) removeReferences(schema *Schema) {
	for name, values := range s.references {
		changed := false
		var modified []BackReference
		for _, value := range values {
			if value.Schema.ID == schema.ID && value.Schema.Version.Path == schema.Version.Path {
				changed = true
				continue
			}
			modified = append(modified, value)
		}

		if changed {
			s.references[name] = modified
		}
	}
}

func (s *Schemas) AddSchema(schema Schema) *Schemas {
	s.Lock()
	defer s.Unlock()
	return s.doAddSchema(schema)
}

func (s *Schemas) doAddSchema(schema Schema) *Schemas {
	s.setupDefaults(&schema)

	if s.AddHook != nil {
		s.AddHook(&schema)
	}

	schemas, ok := s.schemasByPath[schema.Version.Path]
	if !ok {
		schemas = map[string]*Schema{}
		s.schemasByPath[schema.Version.Path] = schemas
		s.versions = append(s.versions, schema.Version)
	}

	if _, ok := schemas[schema.ID]; !ok {
		schemas[schema.ID] = &schema
		s.schemas = append(s.schemas, &schema)

		if !schema.Embed {
			s.addReferences(&schema)
		}
	}

	if schema.Embed {
		s.embed(&schema)
	}

	return s
}

func (s *Schemas) removeEmbed(schema *Schema) {
	target := s.doSchema(&schema.Version, schema.EmbedType, false)
	if target == nil {
		return
	}

	newSchema := *target
	newSchema.ResourceFields = map[string]Field{}

	for k, v := range target.ResourceFields {
		newSchema.ResourceFields[k] = v
	}

	for k := range schema.ResourceFields {
		delete(newSchema.ResourceFields, k)
	}

	s.doRemoveSchema(*target)
	s.doAddSchema(newSchema)
}

func (s *Schemas) embed(schema *Schema) {
	target := s.doSchema(&schema.Version, schema.EmbedType, false)
	if target == nil {
		return
	}

	newSchema := *target
	newSchema.ResourceFields = map[string]Field{}

	for k, v := range target.ResourceFields {
		// We remove the dynamic fields off the existing schema in case
		// they've been removed from the dynamic schema so they won't
		// be accidentally left over
		if !v.DynamicField {
			newSchema.ResourceFields[k] = v
		}
	}
	for k, v := range schema.ResourceFields {
		newSchema.ResourceFields[k] = v
	}

	s.doRemoveSchema(*target)
	s.doAddSchema(newSchema)
}

func (s *Schemas) addReferences(schema *Schema) {
	for name, field := range schema.ResourceFields {
		if !definition.IsReferenceType(field.Type) {
			continue
		}

		refType := definition.SubType(field.Type)
		if !strings.HasPrefix(refType, "/") {
			refType = convert.ToFullReference(schema.Version.Path, refType)
		}

		s.references[refType] = append(s.references[refType], BackReference{
			FieldName: name,
			Schema:    schema,
		})
	}
}

func (s *Schemas) setupDefaults(schema *Schema) {
	schema.Type = "/meta/schemas/schema"
	if schema.ID == "" {
		s.errors = append(s.errors, fmt.Errorf("ID is not set on schema: %v", schema))
		return
	}
	if schema.Version.Path == "" || schema.Version.Version == "" {
		s.errors = append(s.errors, fmt.Errorf("version is not set on schema: %s", schema.ID))
		return
	}
	if schema.PluralName == "" {
		schema.PluralName = name.GuessPluralName(schema.ID)
	}
	if schema.CodeName == "" {
		schema.CodeName = convert.Capitalize(schema.ID)
	}
	if schema.CodeNamePlural == "" {
		schema.CodeNamePlural = name.GuessPluralName(schema.CodeName)
	}
	if schema.BaseType == "" {
		schema.BaseType = schema.ID
	}
}

func (s *Schemas) References(schema *Schema) []BackReference {
	refType := convert.ToFullReference(schema.Version.Path, schema.ID)
	s.Lock()
	defer s.Unlock()
	return s.references[refType]
}

func (s *Schemas) AddMapper(version *APIVersion, schemaID string, mapper Mapper) *Schemas {
	mappers, ok := s.mappers[version.Path]
	if !ok {
		mappers = map[string][]Mapper{}
		s.mappers[version.Path] = mappers
	}

	mappers[schemaID] = append(mappers[schemaID], mapper)
	return s
}

func (s *Schemas) SchemasForVersion(version APIVersion) map[string]*Schema {
	s.Lock()
	defer s.Unlock()
	return s.schemasByPath[version.Path]
}

func (s *Schemas) Versions() []APIVersion {
	return s.versions
}

func (s *Schemas) Schemas() []*Schema {
	return s.schemas
}

func (s *Schemas) mapper(version *APIVersion, name string) []Mapper {
	var (
		path string
	)

	if strings.Contains(name, "/") {
		idx := strings.LastIndex(name, "/")
		path = name[0:idx]
		name = name[idx+1:]
	} else if version != nil {
		path = version.Path
	} else {
		path = "core"
	}

	mappers, ok := s.mappers[path]
	if !ok {
		return nil
	}

	mapper := mappers[name]
	if mapper != nil {
		return mapper
	}

	return nil
}

func (s *Schemas) Schema(version *APIVersion, name string) *Schema {
	return s.doSchema(version, name, true)
}

func (s *Schemas) doSchema(version *APIVersion, name string, lock bool) *Schema {
	var (
		path string
	)

	if strings.Contains(name, "/schemas/") {
		parts := strings.SplitN(name, "/schemas/", 2)
		path = parts[0]
		name = parts[1]
	} else if version != nil {
		path = version.Path
	} else {
		path = "core"
	}

	if lock {
		s.Lock()
	}
	schemas, ok := s.schemasByPath[path]
	if lock {
		s.Unlock()
	}
	if !ok {
		return nil
	}

	schema := schemas[name]
	if schema != nil {
		return schema
	}

	for _, check := range schemas {
		if strings.EqualFold(check.ID, name) || strings.EqualFold(check.PluralName, name) {
			return check
		}
	}

	return nil
}

func (s *Schemas) SubContextVersionForSchema(schema *Schema) *APIVersion {
	fullName := fmt.Sprintf("%s/schemas/%s", schema.Version.Path, schema.ID)
	for _, version := range s.Versions() {
		if version.SubContextSchema == fullName {
			return &version
		}
	}
	return nil
}

type MultiErrors struct {
	Errors []error
}

type Errors struct {
	errors []error
}

func (e *Errors) Add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}

func (e *Errors) Err() error {
	return NewErrors(e.errors...)
}

func NewErrors(inErrors ...error) error {
	var errors []error
	for _, err := range inErrors {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) == 0 {
		return nil
	} else if len(errors) == 1 {
		return errors[0]
	}
	return &MultiErrors{
		Errors: errors,
	}
}

func (m *MultiErrors) Error() string {
	buf := bytes.NewBuffer(nil)
	for _, err := range m.Errors {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(err.Error())
	}

	return buf.String()
}
