package builder

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/definition"
	"k8s.io/apimachinery/pkg/util/validation"
)

var (
	Create         = Operation("create")
	Update         = Operation("update")
	Action         = Operation("action")
	List           = Operation("list")
	ListForCreate  = Operation("listcreate")
	ErrComplexType = errors.New("complex type")
)

type Operation string

func (o Operation) IsList() bool {
	return strings.HasPrefix(string(o), "list")
}

type Builder struct {
	apiContext   *types.APIContext
	Version      *types.APIVersion
	Schemas      *types.Schemas
	RefValidator types.ReferenceValidator
	edit         bool
	export       bool
	yaml         bool
}

func NewBuilder(apiRequest *types.APIContext) *Builder {
	return &Builder{
		apiContext:   apiRequest,
		yaml:         apiRequest.ResponseFormat == "yaml",
		edit:         apiRequest.Option("edit") == "true",
		export:       apiRequest.Option("export") == "true",
		Version:      apiRequest.Version,
		Schemas:      apiRequest.Schemas,
		RefValidator: apiRequest.ReferenceValidator,
	}
}

func (b *Builder) Construct(schema *types.Schema, input map[string]interface{}, op Operation) (map[string]interface{}, error) {
	result, err := b.copyFields(schema, input, op)
	if err != nil {
		return nil, err
	}
	if (op == Create || op == Update) && schema.Validator != nil {
		if err := schema.Validator(b.apiContext, schema, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (b *Builder) copyInputs(schema *types.Schema, input map[string]interface{}, op Operation, result map[string]interface{}) error {
	for fieldName, value := range input {
		field, ok := schema.ResourceFields[fieldName]
		if !ok {
			continue
		}

		if !fieldMatchesOp(field, op) {
			continue
		}

		wasNull := value == nil && (field.Nullable || field.Default == nil)
		value, err := b.convert(field.Type, value, op)
		if err != nil {
			return httperror.WrapFieldAPIError(err, httperror.InvalidFormat, fieldName, err.Error())
		}

		if value != nil || wasNull {
			if !op.IsList() {
				if slice, ok := value.([]interface{}); ok {
					for _, sliceValue := range slice {
						if sliceValue == nil {
							return httperror.NewFieldAPIError(httperror.NotNullable, fieldName, "Individual array values can not be null")
						}
						if err := CheckFieldCriteria(fieldName, field, sliceValue); err != nil {
							return err
						}
					}
				} else {
					if err := CheckFieldCriteria(fieldName, field, value); err != nil {
						return err
					}
				}
			}
			result[fieldName] = value

			if op.IsList() && field.Type == "date" && value != "" && !b.edit {
				ts, err := convert.ToTimestamp(value)
				if err == nil {
					result[fieldName+"TS"] = ts
				}
			}
		}
	}

	if op.IsList() && !b.edit && !b.export {
		if !convert.IsAPIObjectEmpty(input["type"]) {
			result["type"] = input["type"]
		}
		if !convert.IsAPIObjectEmpty(input["id"]) {
			result["id"] = input["id"]
		}
	}

	return nil
}

func (b *Builder) checkDefaultAndRequired(schema *types.Schema, input map[string]interface{}, op Operation, result map[string]interface{}) error {
	for fieldName, field := range schema.ResourceFields {
		val, hasKey := result[fieldName]
		if op == Create && (!hasKey || val == "") && field.Default != nil {
			result[fieldName] = field.Default
		}

		_, hasKey = result[fieldName]
		if op == Create && fieldMatchesOp(field, Create) && field.Required {
			if !hasKey {
				return httperror.NewFieldAPIError(httperror.MissingRequired, fieldName, "")
			}

			if definition.IsArrayType(field.Type) {
				slice, err := b.convertArray(field.Type, result[fieldName], op)
				if err != nil {
					return err
				}
				if len(slice) == 0 {
					return httperror.NewFieldAPIError(httperror.MissingRequired, fieldName, "")
				}
			}
		}

		if op.IsList() && fieldMatchesOp(field, List) && definition.IsReferenceType(field.Type) && !hasKey {
			result[fieldName] = nil
		} else if op.IsList() && fieldMatchesOp(field, List) && !hasKey && field.Default != nil {
			result[fieldName] = field.Default
		}
	}

	if op.IsList() && b.edit {
		b.populateMissingFieldsForEdit(schema, result)
	}

	if op.IsList() && b.export {
		b.dropDefaultsAndReadOnly(schema, result)
	}

	return nil
}

func (b *Builder) dropDefaultsAndReadOnly(schema *types.Schema, result map[string]interface{}) {
	for name, existingVal := range result {
		field, ok := schema.ResourceFields[name]
		if !ok {
			delete(result, name)
		}

		if !field.Create {
			delete(result, name)
			continue
		}

		if field.Default == existingVal {
			delete(result, name)
			continue
		}

		val, err := b.convert(field.Type, nil, List)
		if err == nil && val == existingVal {
			delete(result, name)
			continue
		}

		if convert.IsAPIObjectEmpty(existingVal) {
			delete(result, name)
			continue
		}
	}
}

func (b *Builder) populateMissingFieldsForEdit(schema *types.Schema, result map[string]interface{}) {
	for name, field := range schema.ResourceFields {
		if !field.Update {
			if name != "name" {
				delete(result, name)
			}
			continue
		}

		desc := field.Description
		if len(desc) > 0 {
			desc += " "
		}

		value, hasKey := result[name]
		if hasKey {
			if field.Default != nil && field.Default == value {
				delete(result, name)
				result["zzz#("+desc+")("+field.Type+")"+name] = value
			}
			continue
		}

		if field.Default != nil {
			result["zzz#("+desc+")("+field.Type+")"+name] = field.Default
		} else {
			val, err := b.convert(field.Type, nil, List)
			if err == nil {
				result["zzz#("+desc+")("+field.Type+")"+name] = val
			}
		}
	}
}

func (b *Builder) copyFields(schema *types.Schema, input map[string]interface{}, op Operation) (map[string]interface{}, error) {
	result := map[string]interface{}{}

	if err := b.copyInputs(schema, input, op, result); err != nil {
		return nil, err
	}

	return result, b.checkDefaultAndRequired(schema, input, op, result)
}

func CheckFieldCriteria(fieldName string, field types.Field, value interface{}) error {
	numVal, isNum := value.(int64)
	strVal := ""
	hasStrVal := false

	if value == nil && field.Default != nil {
		value = field.Default
	}

	if value != nil && value != "" {
		hasStrVal = true
		strVal = fmt.Sprint(value)
	}

	if (value == nil || value == "") && !field.Nullable {
		if field.Default == nil {
			return httperror.NewFieldAPIError(httperror.NotNullable, fieldName, "")
		}
	}

	if isNum {
		if field.Min != nil && numVal < *field.Min {
			return httperror.NewFieldAPIError(httperror.MinLimitExceeded, fieldName, "")
		}
		if field.Max != nil && numVal > *field.Max {
			return httperror.NewFieldAPIError(httperror.MaxLimitExceeded, fieldName, "")
		}
	}

	if hasStrVal || value == "" {
		if field.MinLength != nil && int64(len(strVal)) < *field.MinLength {
			return httperror.NewFieldAPIError(httperror.MinLengthExceeded, fieldName, "")
		}
		if field.MaxLength != nil && int64(len(strVal)) > *field.MaxLength {
			return httperror.NewFieldAPIError(httperror.MaxLengthExceeded, fieldName, "")
		}
	}

	if len(field.Options) > 0 {
		if hasStrVal || !field.Nullable {
			found := false
			for _, option := range field.Options {
				if strVal == option {
					found = true
					break
				}
			}

			if !found {
				return httperror.NewFieldAPIError(httperror.InvalidOption, fieldName, "")
			}
		}
	}

	if len(field.ValidChars) > 0 && hasStrVal {
		for _, c := range strVal {
			if !strings.ContainsRune(field.ValidChars, c) {
				return httperror.NewFieldAPIError(httperror.InvalidCharacters, fieldName, "")
			}

		}
	}

	if len(field.InvalidChars) > 0 && hasStrVal {
		if strings.ContainsAny(strVal, field.InvalidChars) {
			return httperror.NewFieldAPIError(httperror.InvalidCharacters, fieldName, "")
		}
	}

	return nil
}

func ConvertSimple(fieldType string, value interface{}, op Operation) (interface{}, error) {
	if value == nil {
		return value, nil
	}

	switch fieldType {
	case "json":
		return value, nil
	case "date":
		v := convert.ToString(value)
		if v == "" {
			return nil, nil
		}
		return v, nil
	case "boolean":
		return convert.ToBool(value), nil
	case "enum":
		return convert.ToString(value), nil
	case "int":
		return convert.ToNumber(value)
	case "float":
		return convert.ToFloat(value)
	case "password":
		return convert.ToString(value), nil
	case "string":
		if op.IsList() {
			return convert.ToStringNoTrim(value), nil
		}
		return convert.ToString(value), nil
	case "dnsLabel":
		str := convert.ToString(value)
		if str == "" {
			return "", nil
		}
		if op == Create || op == Update {
			if errs := validation.IsDNS1123Label(str); len(errs) != 0 {
				return value, httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("invalid value %s: %s", value,
					strings.Join(errs, ",")))
			}
		}
		return str, nil
	case "dnsLabelRestricted":
		str := convert.ToString(value)
		if str == "" {
			return "", nil
		}
		if op == Create || op == Update {
			if errs := validation.IsDNS1035Label(str); len(errs) != 0 {
				return value, httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("invalid value %s: %s", value,
					strings.Join(errs, ",")))
			}
		}
		return str, nil
	case "hostname":
		str := convert.ToString(value)
		if str == "" {
			return "", nil
		}
		if op == Create || op == Update {
			if errs := validation.IsDNS1123Subdomain(str); len(errs) != 0 {
				return value, httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("invalid value %s: %s", value,
					strings.Join(errs, ",")))
			}
		}
		return str, nil
	case "intOrString":
		num, err := convert.ToNumber(value)
		if err == nil {
			return num, nil
		}
		return convert.ToString(value), nil
	case "base64":
		return convert.ToString(value), nil
	case "reference":
		return convert.ToString(value), nil
	}

	return nil, ErrComplexType
}

func (b *Builder) convert(fieldType string, value interface{}, op Operation) (interface{}, error) {
	if value == nil {
		return value, nil
	}

	switch {
	case definition.IsMapType(fieldType):
		return b.convertMap(fieldType, value, op)
	case definition.IsArrayType(fieldType):
		return b.convertArray(fieldType, value, op)
	case definition.IsReferenceType(fieldType):
		return b.convertReferenceType(fieldType, value)
	}

	newValue, err := ConvertSimple(fieldType, value, op)
	if err == ErrComplexType {
		return b.convertType(fieldType, value, op)
	}
	return newValue, err
}

func (b *Builder) convertType(fieldType string, value interface{}, op Operation) (interface{}, error) {
	schema := b.Schemas.Schema(b.Version, fieldType)
	if schema == nil {
		return nil, httperror.NewAPIError(httperror.InvalidType, "Failed to find type "+fieldType)
	}

	mapValue, ok := value.(map[string]interface{})
	if !ok {
		return nil, httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("Value can not be converted to type %s: %v", fieldType, value))
	}

	return b.Construct(schema, mapValue, op)
}

func (b *Builder) convertReferenceType(fieldType string, value interface{}) (string, error) {
	subType := definition.SubType(fieldType)
	strVal := convert.ToString(value)
	if b.RefValidator != nil && !b.RefValidator.Validate(subType, strVal) {
		return "", httperror.NewAPIError(httperror.InvalidReference, fmt.Sprintf("Not found type: %s id: %s", subType, strVal))
	}
	return strVal, nil
}

func (b *Builder) convertArray(fieldType string, value interface{}, op Operation) ([]interface{}, error) {
	if strSliceValue, ok := value.([]string); ok {
		// Form data will be []string
		var result []interface{}
		for _, value := range strSliceValue {
			result = append(result, value)
		}
		return result, nil
	}

	sliceValue, ok := value.([]interface{})
	if !ok {
		return nil, nil
	}

	var result []interface{}
	subType := definition.SubType(fieldType)

	for _, value := range sliceValue {
		val, err := b.convert(subType, value, op)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}

	return result, nil
}

func (b *Builder) convertMap(fieldType string, value interface{}, op Operation) (map[string]interface{}, error) {
	mapValue, ok := value.(map[string]interface{})
	if !ok {
		return nil, nil
	}

	result := map[string]interface{}{}
	subType := definition.SubType(fieldType)

	for key, value := range mapValue {
		val, err := b.convert(subType, value, op)
		if err != nil {
			return nil, httperror.WrapAPIError(err, httperror.InvalidFormat, err.Error())
		}
		result[key] = val
	}

	return result, nil
}

func fieldMatchesOp(field types.Field, op Operation) bool {
	switch op {
	case Create:
		return field.Create
	case Update:
		return field.Update
	case List:
		if field.Type == "password" {
			return false
		}
		return !field.WriteOnly
	case ListForCreate:
		if field.Type == "password" {
			return false
		}
		return true
	default:
		return false
	}
}
