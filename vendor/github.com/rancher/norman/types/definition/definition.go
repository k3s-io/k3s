package definition

import (
	"strings"

	"github.com/rancher/norman/types/convert"
)

func IsMapType(fieldType string) bool {
	return strings.HasPrefix(fieldType, "map[") && strings.HasSuffix(fieldType, "]")
}

func IsArrayType(fieldType string) bool {
	return strings.HasPrefix(fieldType, "array[") && strings.HasSuffix(fieldType, "]")
}

func IsReferenceType(fieldType string) bool {
	return strings.HasPrefix(fieldType, "reference[") && strings.HasSuffix(fieldType, "]")
}

func HasReferenceType(fieldType string) bool {
	return strings.Contains(fieldType, "reference[")
}

func SubType(fieldType string) string {
	i := strings.Index(fieldType, "[")
	if i <= 0 || i >= len(fieldType)-1 {
		return fieldType
	}

	return fieldType[i+1 : len(fieldType)-1]
}

func GetType(data map[string]interface{}) string {
	return GetShortTypeFromFull(GetFullType(data))
}

func GetShortTypeFromFull(fullType string) string {
	parts := strings.Split(fullType, "/")
	return parts[len(parts)-1]
}

func GetFullType(data map[string]interface{}) string {
	return convert.ToString(data["type"])
}
