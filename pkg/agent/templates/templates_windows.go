//go:build windows
// +build windows

package templates

import (
	"encoding/json"
	"net/url"
	"strings"
	"text/template"
)

// Windows config templates need named pipe addresses fixed up
var templateFuncs = template.FuncMap{
	"deschemify": func(s string) string {
		if strings.HasPrefix(s, "npipe:") {
			u, err := url.Parse(s)
			if err != nil {
				return ""
			}
			return u.Path
		}
		return s
	},
	"toJson": func(v interface{}) string {
		output, _ := json.Marshal(v)
		return string(output)
	},
}
