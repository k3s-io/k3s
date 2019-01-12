package handler

import (
	"net/url"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
)

func ErrorHandler(request *types.APIContext, err error) {
	var error *httperror.APIError
	if apiError, ok := err.(*httperror.APIError); ok {
		if apiError.Cause != nil {
			url, _ := url.PathUnescape(request.Request.URL.String())
			if url == "" {
				url = request.Request.URL.String()
			}
			logrus.Errorf("API error response %v for %v %v. Cause: %v", apiError.Code.Status, request.Request.Method,
				url, apiError.Cause)
		}
		error = apiError
	} else {
		logrus.Errorf("Unknown error: %v", err)
		error = &httperror.APIError{
			Code:    httperror.ServerError,
			Message: err.Error(),
		}
	}

	data := toError(error)
	request.WriteResponse(error.Code.Status, data)
}

func toError(apiError *httperror.APIError) map[string]interface{} {
	e := map[string]interface{}{
		"type":    "/meta/schemas/error",
		"status":  apiError.Code.Status,
		"code":    apiError.Code.Code,
		"message": apiError.Message,
	}
	if apiError.FieldName != "" {
		e["fieldName"] = apiError.FieldName
	}

	return e
}
