package util

import (
	"net/http"

	"github.com/k3s-io/k3s/pkg/generated/clientset/versioned/scheme"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
)

var ErrNotReady = errors.New("apiserver not ready")

// SendError sends a properly formatted error response
func SendError(err error, resp http.ResponseWriter, req *http.Request, status ...int) {
	var code int
	if len(status) == 1 {
		code = status[0]
	}
	if code == 0 || code == http.StatusOK {
		code = http.StatusInternalServerError
	}

	// Don't log "apiserver not ready" errors, they are frequent during startup
	if !errors.Is(err, ErrNotReady) {
		logrus.Errorf("Sending HTTP %d response to %s: %v", code, req.RemoteAddr, err)
	}

	var serr *apierrors.StatusError
	switch code {
	case http.StatusBadRequest:
		serr = apierrors.NewBadRequest(err.Error())
	case http.StatusUnauthorized:
		serr = apierrors.NewUnauthorized(err.Error())
	case http.StatusForbidden:
		serr = newForbidden(err)
	case http.StatusInternalServerError:
		serr = apierrors.NewInternalError(err)
	case http.StatusBadGateway:
		serr = newBadGateway(err)
	case http.StatusServiceUnavailable:
		serr = apierrors.NewServiceUnavailable(err.Error())
	default:
		serr = apierrors.NewGenericServerResponse(code, req.Method, schema.GroupResource{}, req.URL.Path, err.Error(), 0, true)
	}

	responsewriters.ErrorNegotiated(serr, scheme.Codecs.WithoutConversion(), schema.GroupVersion{}, resp, req)
}

func newForbidden(err error) *apierrors.StatusError {
	return &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusForbidden,
			Reason:  metav1.StatusReasonForbidden,
			Message: err.Error(),
		}}
}

func newBadGateway(err error) *apierrors.StatusError {
	return &apierrors.StatusError{
		ErrStatus: metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusBadGateway,
			Reason:  metav1.StatusReasonInternalError,
			Message: err.Error(),
		}}
}
