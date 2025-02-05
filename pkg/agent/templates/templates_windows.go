//go:build windows
// +build windows

package templates

import (
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
}
