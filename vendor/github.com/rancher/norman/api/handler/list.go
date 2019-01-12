package handler

import (
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/types"
)

func ListHandler(request *types.APIContext, next types.RequestHandler) error {
	var (
		err  error
		data interface{}
	)

	store := request.Schema.Store
	if store == nil {
		return httperror.NewAPIError(httperror.NotFound, "no store found")
	}

	if request.ID == "" {
		opts := parse.QueryOptions(request, request.Schema)
		// Save the pagination on the context so it's not reset later
		request.Pagination = opts.Pagination
		data, err = store.List(request, request.Schema, &opts)
	} else if request.Link == "" {
		data, err = store.ByID(request, request.Schema, request.ID)
	} else {
		_, err = store.ByID(request, request.Schema, request.ID)
		if err != nil {
			return err
		}
		return request.Schema.LinkHandler(request, nil)
	}

	if err != nil {
		return err
	}

	request.WriteResponse(http.StatusOK, data)
	return nil
}
