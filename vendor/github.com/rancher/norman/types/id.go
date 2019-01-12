package types

import (
	"fmt"
	"regexp"
	"strings"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

var (
	lowerChars = regexp.MustCompile("[a-z]+")
)

func GenerateName(typeName string) string {
	base := typeName[0:1] + lowerChars.ReplaceAllString(typeName[1:], "")
	last := utilrand.String(5)
	return fmt.Sprintf("%s-%s", strings.ToLower(base), last)
}
