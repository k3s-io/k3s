package generic

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
)

type handlerEntry struct {
	generation int
	name       string
	handler    Handler
}

type Handlers struct {
	handlers []handlerEntry
}

func (h *Handlers) Handle(key string, obj runtime.Object) (runtime.Object, error) {
	var (
		errs errors
	)

	for _, handler := range h.handlers {
		newObj, err := handler.handler(key, obj)
		if err != nil {
			errs = append(errs, &handlerError{
				HandlerName: handler.name,
				Err:         err,
			})
		}
		if newObj != nil {
			obj = newObj
		}
	}

	return obj, errs.ToErr()
}

type errors []error

func (e errors) Error() string {
	buf := strings.Builder{}
	for _, err := range e {
		if buf.Len() > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(err.Error())
	}
	return buf.String()
}

func (e errors) ToErr() error {
	switch len(e) {
	case 0:
		return nil
	case 1:
		return e[0]
	default:
		return e
	}
}

type handlerError struct {
	HandlerName string
	Err         error
}

func (h handlerError) Error() string {
	return fmt.Sprintf("handler %s: %v", h.HandlerName, h.Err)
}
