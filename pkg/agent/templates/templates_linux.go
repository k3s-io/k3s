//go:build linux

package templates

import (
	"encoding/json"
	"text/template"
)

// Linux config templates do not need fixups
var templateFuncs = template.FuncMap{
	"deschemify": func(s string) string {
		return s
	},
	"toJson": func(v interface{}) string {
		output, _ := json.Marshal(v)
		return string(output)
	},
}
