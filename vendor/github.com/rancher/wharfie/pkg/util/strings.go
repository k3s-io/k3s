package util

import "strings"

// HasSuffixI returns true if string s has any of the given suffixes, ignoring case.
func HasSuffixI(s string, suffixes ...string) bool {
	s = strings.ToLower(s)
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, strings.ToLower(suffix)) {
			return true
		}
	}
	return false
}
