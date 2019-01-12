package api

import "github.com/rancher/norman/types"

type ResponseWriter interface {
	Write(apiContext *types.APIContext, code int, obj interface{})
}
