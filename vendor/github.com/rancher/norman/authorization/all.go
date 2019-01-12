package authorization

import (
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
)

type AllAccess struct {
}

func (*AllAccess) CanCreate(apiContext *types.APIContext, schema *types.Schema) error {
	if slice.ContainsString(schema.CollectionMethods, http.MethodPost) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not create "+schema.ID)
}

func (*AllAccess) CanGet(apiContext *types.APIContext, schema *types.Schema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodGet) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not get "+schema.ID)
}

func (*AllAccess) CanList(apiContext *types.APIContext, schema *types.Schema) error {
	if slice.ContainsString(schema.CollectionMethods, http.MethodGet) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not list "+schema.ID)
}

func (*AllAccess) CanUpdate(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodPut) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not update "+schema.ID)
}

func (*AllAccess) CanDelete(apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	if slice.ContainsString(schema.ResourceMethods, http.MethodDelete) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not delete "+schema.ID)
}

func (*AllAccess) CanDo(apiGroup, resource, verb string, apiContext *types.APIContext, obj map[string]interface{}, schema *types.Schema) error {
	if slice.ContainsString(schema.ResourceMethods, verb) {
		return nil
	}
	return httperror.NewAPIError(httperror.PermissionDenied, "can not perform "+verb+" "+schema.ID)
}

func (*AllAccess) Filter(apiContext *types.APIContext, schema *types.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	return obj
}

func (*AllAccess) FilterList(apiContext *types.APIContext, schema *types.Schema, obj []map[string]interface{}, context map[string]string) []map[string]interface{} {
	return obj
}
