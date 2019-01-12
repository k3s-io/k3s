package generator

import (
	"net/http"
	"strings"
	"text/template"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

func funcs() template.FuncMap {
	return template.FuncMap{
		"capitalize":          convert.Capitalize,
		"unCapitalize":        convert.Uncapitalize,
		"upper":               strings.ToUpper,
		"toLower":             strings.ToLower,
		"hasGet":              hasGet,
		"hasPost":             hasPost,
		"getCollectionOutput": getCollectionOutput,
	}
}

func addUnderscore(input string) string {
	return strings.ToLower(underscoreRegexp.ReplaceAllString(input, `${1}_${2}`))
}

func hasGet(schema *types.Schema) bool {
	return contains(schema.CollectionMethods, http.MethodGet)
}

func hasPost(schema *types.Schema) bool {
	return contains(schema.CollectionMethods, http.MethodPost)
}

func contains(list []string, needle string) bool {
	for _, i := range list {
		if i == needle {
			return true
		}
	}
	return false
}

func getCollectionOutput(output, codeName string) string {
	if output == "collection" {
		return codeName + "Collection"
	}
	return convert.Capitalize(output)
}
