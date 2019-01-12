package name

import "strings"

func GuessPluralName(name string) string {
	if name == "" {
		return name
	}

	if strings.EqualFold(name, "Endpoints") {
		return name
	}

	if suffix(name, "s") || suffix(name, "ch") || suffix(name, "x") {
		return name + "es"
	}

	if suffix(name, "y") && len(name) > 2 && !strings.ContainsAny(name[len(name)-2:len(name)-1], "[aeiou]") {
		return name[0:len(name)-1] + "ies"
	}

	return name + "s"
}

func suffix(str, end string) bool {
	return strings.HasSuffix(str, end)
}
