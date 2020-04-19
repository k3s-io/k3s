package openapi

import (
	"fmt"

	types "github.com/rancher/wrangler/pkg/schemas"
	"github.com/rancher/wrangler/pkg/schemas/definition"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

func MustGenerate(obj interface{}) *v1beta1.JSONSchemaProps {
	if obj == nil {
		return nil
	}
	result, err := ToOpenAPIFromStruct(obj)
	if err != nil {
		panic(err)
	}
	return result
}

func ToOpenAPIFromStruct(obj interface{}) (*v1beta1.JSONSchemaProps, error) {
	schemas := types.EmptySchemas()
	schema, err := schemas.Import(obj)
	if err != nil {
		return nil, err
	}

	return toOpenAPI(schema.ID, schemas)
}

func toOpenAPI(name string, schemas *types.Schemas) (*v1beta1.JSONSchemaProps, error) {
	schema := schemas.Schema(name)
	if schema == nil {
		return nil, fmt.Errorf("failed to find schema: %s", name)
	}

	newSchema := schema.DeepCopy()
	if newSchema.InternalSchema != nil {
		newSchema = newSchema.InternalSchema.DeepCopy()
	}
	delete(newSchema.ResourceFields, "kind")
	delete(newSchema.ResourceFields, "apiVersion")
	delete(newSchema.ResourceFields, "metadata")
	return parseSchema(newSchema, schemas)
}

func parseSchema(schema *types.Schema, schemas *types.Schemas) (*v1beta1.JSONSchemaProps, error) {
	jsp := &v1beta1.JSONSchemaProps{
		Description: schema.Description,
		Type:        "object",
		Properties:  map[string]v1beta1.JSONSchemaProps{},
	}

	for name, f := range schema.ResourceFields {
		fieldJSP := v1beta1.JSONSchemaProps{
			Description: f.Description,
			Nullable:    f.Nullable,
			MinLength:   f.MinLength,
			MaxLength:   f.MaxLength,
		}

		if len(f.Options) > 0 {
			for _, opt := range f.Options {
				fieldJSP.Enum = append(fieldJSP.Enum, v1beta1.JSON{
					Raw: []byte(opt),
				})
			}
		}

		if len(f.InvalidChars) > 0 {
			fieldJSP.Pattern = fmt.Sprintf("^[^%s]*$", f.InvalidChars)
		}

		if len(f.ValidChars) > 0 {
			fieldJSP.Pattern = fmt.Sprintf("^[%s]*$", f.ValidChars)
		}

		if f.Min != nil {
			fl := float64(*f.Min)
			fieldJSP.Minimum = &fl
		}

		if f.Max != nil {
			fl := float64(*f.Max)
			fieldJSP.Maximum = &fl
		}

		//  default is not support by k8s
		//
		//if f.Default != nil {
		//	bytes, err := json.Marshal(f.Default)
		//	if err != nil {
		//		return nil, err
		//	}
		//	fieldJSP.Default = &v1beta1.JSON{
		//		Raw: bytes,
		//	}
		//}

		if f.Required {
			fieldJSP.Required = append(fieldJSP.Required, name)
		}

		if definition.IsMapType(f.Type) {
			fieldJSP.Type = "object"
			subType := definition.SubType(f.Type)

			subType, schema, err := typeAndSchema(subType, schemas)
			if err != nil {
				return nil, err
			}

			if schema == nil {
				fieldJSP.AdditionalProperties = &v1beta1.JSONSchemaPropsOrBool{
					Schema: &v1beta1.JSONSchemaProps{
						Type: subType,
					},
				}
			} else {
				subObject, err := parseSchema(schema, schemas)
				if err != nil {
					return nil, err
				}

				fieldJSP.AdditionalProperties = &v1beta1.JSONSchemaPropsOrBool{
					Schema: subObject,
				}
			}
		} else if definition.IsArrayType(f.Type) {
			fieldJSP.Type = "array"
			subType := definition.SubType(f.Type)

			subType, schema, err := typeAndSchema(subType, schemas)
			if err != nil {
				return nil, err
			}

			if schema == nil {
				fieldJSP.Items = &v1beta1.JSONSchemaPropsOrArray{
					Schema: &v1beta1.JSONSchemaProps{
						Type: subType,
					},
				}
			} else {
				subObject, err := parseSchema(schema, schemas)
				if err != nil {
					return nil, err
				}

				fieldJSP.Items = &v1beta1.JSONSchemaPropsOrArray{
					Schema: subObject,
				}
			}
		} else {
			typeName, schema, err := typeAndSchema(f.Type, schemas)
			if err != nil {
				return nil, err
			}
			if schema == nil {
				fieldJSP.Type = typeName
			} else {
				fieldJSP.Type = "object"
				subObject, err := parseSchema(schema, schemas)
				if err != nil {
					return nil, err
				}
				fieldJSP.Properties = subObject.Properties
			}
		}

		jsp.Properties[name] = fieldJSP
	}

	return jsp, nil
}

func typeAndSchema(typeName string, schemas *types.Schemas) (string, *types.Schema, error) {
	switch typeName {
	// TODO: in v1 set the x- header for this
	case "intOrString":
		return "string", nil, nil
	case "int":
		return "integer", nil, nil
	case "float":
		return "number", nil, nil
	case "string":
		return "string", nil, nil
	case "date":
		return "string", nil, nil
	case "enum":
		return "string", nil, nil
	case "password":
		return "string", nil, nil
	case "hostname":
		return "string", nil, nil
	case "boolean":
		return "boolean", nil, nil
	case "json":
		return "object", nil, nil
	}

	if definition.IsReferenceType(typeName) {
		return "string", nil, nil
	}

	if definition.IsArrayType(typeName) {
		return "array", nil, nil
	}

	schema := schemas.Schema(typeName)
	if schema == nil {
		return "", nil, fmt.Errorf("failed to find schema %s", typeName)
	}
	if schema.InternalSchema != nil {
		return "", schema.InternalSchema, nil
	}
	return "", schema, nil
}
