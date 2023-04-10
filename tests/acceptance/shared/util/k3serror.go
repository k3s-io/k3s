package util

import (
	"fmt"
)

// K3sError is a custom error type that includes the source of the error.
type K3sError struct {
	ErrorSource string
	Message     interface{}
	Err         error
}

// Error returns the error message implementing the error interface.
func (k K3sError) Error() string {
	if k.Err != nil {
		return fmt.Sprintf("ErrorSource: %s, Message: %v, Err: %s",
			k.ErrorSource, k.Message, k.Err)
	}

	return fmt.Sprintf("ErrorSource: %s, Message: %v",
		k.ErrorSource, k.Message)
}
