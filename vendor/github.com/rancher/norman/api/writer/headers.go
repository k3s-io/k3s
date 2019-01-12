package writer

import (
	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/types"
)

func AddCommonResponseHeader(apiContext *types.APIContext) error {
	addExpires(apiContext)
	return addSchemasHeader(apiContext)
}

func addSchemasHeader(apiContext *types.APIContext) error {
	schema := apiContext.Schemas.Schema(&builtin.Version, "schema")
	if schema == nil {
		return nil
	}

	version := apiContext.SchemasVersion
	if version == nil {
		version = apiContext.Version
	}

	apiContext.Response.Header().Set("X-Api-Schemas", apiContext.URLBuilder.Collection(schema, version))
	return nil
}

func addExpires(apiContext *types.APIContext) {
	apiContext.Response.Header().Set("Expires", "Wed 24 Feb 1982 18:42:00 GMT")
}
