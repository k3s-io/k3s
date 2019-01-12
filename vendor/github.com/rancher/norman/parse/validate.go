package parse

import (
	"fmt"
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
)

var (
	supportedMethods = map[string]bool{
		http.MethodPost:   true,
		http.MethodGet:    true,
		http.MethodPut:    true,
		http.MethodDelete: true,
	}
)

func ValidateMethod(request *types.APIContext) error {
	if request.Action != "" && request.Method == http.MethodPost {
		return nil
	}

	if !supportedMethods[request.Method] {
		return httperror.NewAPIError(httperror.MethodNotAllowed, fmt.Sprintf("Method %s not supported", request.Method))
	}

	if request.Type == "" || request.Schema == nil || request.Link != "" {
		return nil
	}

	allowed := request.Schema.ResourceMethods
	if request.ID == "" {
		allowed = request.Schema.CollectionMethods
	}

	for _, method := range allowed {
		if method == request.Method {
			return nil
		}
	}

	return httperror.NewAPIError(httperror.MethodNotAllowed, fmt.Sprintf("Method %s not supported", request.Method))
}
