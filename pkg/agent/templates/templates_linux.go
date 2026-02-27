//go:build linux

package templates

import (
	"encoding/json"
	"path/filepath"
	"text/template"
)

// Linux config templates do not need fixups
var templateFuncs = template.FuncMap{
	"deschemify": func(s string) string {
		return s
	},
	"toJson": func(v any) string {
		output, _ := json.Marshal(v)
		return string(output)
	},
	"filepathjoin": filepath.Join,
}
