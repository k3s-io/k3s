package convert

import "fmt"

func ToReference(typeName string) string {
	return fmt.Sprintf("reference[%s]", typeName)
}

func ToFullReference(path, typeName string) string {
	return fmt.Sprintf("reference[%s/schemas/%s]", path, typeName)
}
