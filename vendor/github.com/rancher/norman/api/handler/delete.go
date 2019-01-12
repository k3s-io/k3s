package handler

import (
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
)

func DeleteHandler(request *types.APIContext, next types.RequestHandler) error {
	store := request.Schema.Store
	if store == nil {
		return httperror.NewAPIError(httperror.NotFound, "no store found")
	}

	obj, err := store.Delete(request, request.Schema, request.ID)
	if err != nil {
		return err
	}

	if obj == nil {
		request.WriteResponse(http.StatusNoContent, nil)
	} else {
		request.WriteResponse(http.StatusOK, obj)
	}
	return nil
}
