package convert

import (
	"regexp"
	"strings"
)

var (
	splitRegexp = regexp.MustCompile("[[:space:]]*,[[:space:]]*")
)

func ToValuesSlice(value string) []string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
		return splitRegexp.Split(value[1:len(value)-1], -1)
	}
	return []string{value}
}
