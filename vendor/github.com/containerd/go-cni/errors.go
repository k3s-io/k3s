package cni

import (
	"github.com/pkg/errors"
)

var (
	ErrCNINotInitialized = errors.New("cni plugin not initialized")
	ErrInvalidConfig     = errors.New("invalid cni config")
	ErrNotFound          = errors.New("not found")
	ErrRead              = errors.New("failed to read config file")
	ErrInvalidResult     = errors.New("invalid result")
	ErrLoad              = errors.New("failed to load cni config")
)

// IsCNINotInitialized returns true if the error is due cni config not being intialized
func IsCNINotInitialized(err error) bool {
	return errors.Cause(err) == ErrCNINotInitialized
}

// IsInvalidConfig returns true if the error is invalid cni config
func IsInvalidConfig(err error) bool {
	return errors.Cause(err) == ErrInvalidConfig
}

// IsNotFound returns true if the error is due to a missing config or result
func IsNotFound(err error) bool {
	return errors.Cause(err) == ErrNotFound
}

// IsReadFailure return true if the error is a config read failure
func IsReadFailure(err error) bool {
	return errors.Cause(err) == ErrRead
}

// IsInvalidResult return true if the error is due to invalid cni result
func IsInvalidResult(err error) bool {
	return errors.Cause(err) == ErrInvalidResult
}
