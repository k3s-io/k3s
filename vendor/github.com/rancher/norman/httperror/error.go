package httperror

import (
	"fmt"
)

var (
	Unauthorized     = ErrorCode{"Unauthorized", 401}
	PermissionDenied = ErrorCode{"PermissionDenied", 403}
	NotFound         = ErrorCode{"NotFound", 404}
	MethodNotAllowed = ErrorCode{"MethodNotAllow", 405}
	Conflict         = ErrorCode{"Conflict", 409}

	InvalidDateFormat  = ErrorCode{"InvalidDateFormat", 422}
	InvalidFormat      = ErrorCode{"InvalidFormat", 422}
	InvalidReference   = ErrorCode{"InvalidReference", 422}
	NotNullable        = ErrorCode{"NotNullable", 422}
	NotUnique          = ErrorCode{"NotUnique", 422}
	MinLimitExceeded   = ErrorCode{"MinLimitExceeded", 422}
	MaxLimitExceeded   = ErrorCode{"MaxLimitExceeded", 422}
	MinLengthExceeded  = ErrorCode{"MinLengthExceeded", 422}
	MaxLengthExceeded  = ErrorCode{"MaxLengthExceeded", 422}
	InvalidOption      = ErrorCode{"InvalidOption", 422}
	InvalidCharacters  = ErrorCode{"InvalidCharacters", 422}
	MissingRequired    = ErrorCode{"MissingRequired", 422}
	InvalidCSRFToken   = ErrorCode{"InvalidCSRFToken", 422}
	InvalidAction      = ErrorCode{"InvalidAction", 422}
	InvalidBodyContent = ErrorCode{"InvalidBodyContent", 422}
	InvalidType        = ErrorCode{"InvalidType", 422}
	ActionNotAvailable = ErrorCode{"ActionNotAvailable", 404}
	InvalidState       = ErrorCode{"InvalidState", 422}

	ServerError        = ErrorCode{"ServerError", 500}
	ClusterUnavailable = ErrorCode{"ClusterUnavailable", 503}
)

type ErrorCode struct {
	Code   string
	Status int
}

func (e ErrorCode) String() string {
	return fmt.Sprintf("%s %d", e.Code, e.Status)
}

type APIError struct {
	Code      ErrorCode
	Message   string
	Cause     error
	FieldName string
}

func NewAPIErrorLong(status int, code, message string) error {
	return NewAPIError(ErrorCode{
		Code:   code,
		Status: status,
	}, message)
}

func NewAPIError(code ErrorCode, message string) error {
	return &APIError{
		Code:    code,
		Message: message,
	}
}

func NewFieldAPIError(code ErrorCode, fieldName, message string) error {
	return &APIError{
		Code:      code,
		Message:   message,
		FieldName: fieldName,
	}
}

// WrapFieldAPIError will cause the API framework to log the underlying err before returning the APIError as a response.
// err WILL NOT be in the API response
func WrapFieldAPIError(err error, code ErrorCode, fieldName, message string) error {
	return &APIError{
		Cause:     err,
		Code:      code,
		Message:   message,
		FieldName: fieldName,
	}
}

// WrapAPIError will cause the API framework to log the underlying err before returning the APIError as a response.
// err WILL NOT be in the API response
func WrapAPIError(err error, code ErrorCode, message string) error {
	return &APIError{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

func (a *APIError) Error() string {
	if a.FieldName != "" {
		return fmt.Sprintf("%s=%s: %s", a.FieldName, a.Code, a.Message)
	}
	return fmt.Sprintf("%s: %s", a.Code, a.Message)
}

func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}

func IsConflict(err error) bool {
	if apiError, ok := err.(*APIError); ok {
		return apiError.Code.Status == 409
	}

	return false
}
