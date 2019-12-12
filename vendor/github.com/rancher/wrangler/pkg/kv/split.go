package kv

import "strings"

// Like split but if there is only one item return "", item
func RSplit(s, sep string) (string, string) {
	parts := strings.SplitN(s, sep, 2)
	if len(parts) == 1 {
		return "", strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(safeIndex(parts, 1))
}

func Split(s, sep string) (string, string) {
	parts := strings.SplitN(s, sep, 2)
	return strings.TrimSpace(parts[0]), strings.TrimSpace(safeIndex(parts, 1))
}

func SplitMap(s, sep string) map[string]string {
	return SplitMapFromSlice(strings.Split(s, sep))
}

func SplitMapFromSlice(parts []string) map[string]string {
	result := map[string]string{}
	for _, part := range parts {
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
