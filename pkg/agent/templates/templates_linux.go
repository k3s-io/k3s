//go:build linux

package templates

import (
	"text/template"
)

// Linux config templates do not need fixups
var templateFuncs = template.FuncMap{
	"deschemify": func(s string) string {
		return s
	},
}
