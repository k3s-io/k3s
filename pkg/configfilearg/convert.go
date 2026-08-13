package configfilearg

import (
	"fmt"
	"strings"
	"time"
)

// Cribbed from https://github.com/rancher/wrangler/blob/v3.7.0/pkg/data/convert/convert.go
// We only need the string conversions, so this omits all the other type handling
// including those that operate on metav1/unstructured. This saves us a bunch of
// unnecessary imports in the self-extracting wrapper binary.

func Singular(value any) any {
	if slice, ok := value.([]string); ok {
		if len(slice) == 0 {
			return nil
		}
		return slice[0]
	}
	if slice, ok := value.([]any); ok {
		if len(slice) == 0 {
			return nil
		}
		return slice[0]
	}
	return value
}

func ToStringNoTrim(value any) string {
	if t, ok := value.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	single := Singular(value)
	if single == nil {
		return ""
	}
	return fmt.Sprint(single)
}

func ToString(value any) string {
	return strings.TrimSpace(ToStringNoTrim(value))
}
