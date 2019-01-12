package handler

import (
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
)

func CreateHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	var err error

	data, err := ParseAndValidateBody(apiContext, true)
	if err != nil {
		return err
	}

	store := apiContext.Schema.Store
	if store == nil {
		return httperror.NewAPIError(httperror.NotFound, "no store found")
	}

	data, err = store.Create(apiContext, apiContext.Schema, data)
	if err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusCreated, data)
	return nil
}
