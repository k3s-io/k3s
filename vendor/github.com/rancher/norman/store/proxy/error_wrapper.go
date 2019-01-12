package proxy

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

type errorStore struct {
	types.Store
}

func (e *errorStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := e.Store.ByID(apiContext, schema, id)
	return data, translateError(err)
}

func (e *errorStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	data, err := e.Store.List(apiContext, schema, opt)
	return data, translateError(err)
}

func (e *errorStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	data, err := e.Store.Create(apiContext, schema, data)
	return data, translateError(err)

}

func (e *errorStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	data, err := e.Store.Update(apiContext, schema, data, id)
	return data, translateError(err)

}

func (e *errorStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	data, err := e.Store.Delete(apiContext, schema, id)
	return data, translateError(err)

}

func (e *errorStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	data, err := e.Store.Watch(apiContext, schema, opt)
	return data, translateError(err)
}

func translateError(err error) error {
	if apiError, ok := err.(errors.APIStatus); ok {
		status := apiError.Status()
		return httperror.NewAPIErrorLong(int(status.Code), string(status.Reason), status.Message)
	}
	return err
}
