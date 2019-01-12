package kv

import "strings"

func Split(s, sep string) (string, string) {
	parts := strings.SplitN(s, sep, 2)
	return strings.TrimSpace(parts[0]), strings.TrimSpace(safeIndex(parts, 1))
}

func SplitMap(s, sep string) map[string]string {
	result := map[string]string{}
	for _, part := range strings.Split(s, sep) {
		k, v := Split(part, "=")
		result[k] = v
	}
	return result
}

func safeIndex(parts []string, idx int) string {
	if len(parts) <= idx {
		return ""
	}
	return parts[idx]
}
