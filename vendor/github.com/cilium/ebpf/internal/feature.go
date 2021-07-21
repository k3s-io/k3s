package internal

import (
	"errors"
	"fmt"
	"sync"
)

// ErrNotSupported indicates that a feature is not supported by the current kernel.
var ErrNotSupported = errors.New("not supported")

// UnsupportedFeatureError is returned by FeatureTest() functions.
type UnsupportedFeatureError struct {
	// The minimum Linux mainline version required for this feature.
	// Used for the error string, and for sanity checking during testing.
	MinimumVersion Version

	// The name of the feature that isn't supported.
	Name string
}

func (ufe *UnsupportedFeatureError) Error() string {
	if ufe.MinimumVersion.Unspecified() {
		return fmt.Sprintf("%s not supported", ufe.Name)
	}
	return fmt.Sprintf("%s not supported (requires >= %s)", ufe.Name, ufe.MinimumVersion)
}

// Is indicates that UnsupportedFeatureError is ErrNotSupported.
func (ufe *UnsupportedFeatureError) Is(target error) bool {
	return target == ErrNotSupported
}

type featureTest struct {
	sync.RWMutex
	successful bool
	result     error
}

// FeatureTestFn is used to determine whether the kernel supports
// a certain feature.
//
// The return values have the following semantics:
//
//   err == ErrNotSupported: the feature is not available
//   err == nil: the feature is available
//   err != nil: the test couldn't be executed
type FeatureTestFn func() error

// FeatureTest wraps a function so that it is run at most once.
//
// name should identify the tested feature, while version must be in the
// form Major.Minor[.Patch].
//
// Returns an error wrapping ErrNotSupported if the feature is not supported.
func FeatureTest(name, version string, fn FeatureTestFn) func() error {
	v, err := NewVersion(version)
	if err != nil {
		return func() error { return err }
	}

	ft := new(featureTest)
	return func() error {
		ft.RLock()
		if ft.successful {
			defer ft.RUnlock()
			return ft.result
		}
		ft.RUnlock()
		ft.Lock()
		defer ft.Unlock()
		// check one more time on the off
		// chance that two go routines
		// were able to call into the write
		// lock
		if ft.successful {
			return ft.result
		}
		err := fn()
		switch {
		case errors.Is(err, ErrNotSupported):
			ft.result = &UnsupportedFeatureError{
				MinimumVersion: v,
				Name:           name,
			}
			fallthrough

		case err == nil:
			ft.successful = true

		default:
			// We couldn't execute the feature test to a point
			// where it could make a determination.
			// Don't cache the result, just return it.
			return fmt.Errorf("detect support for %s: %w", name, err)
		}

		return ft.result
	}
}

// A Version in the form Major.Minor.Patch.
type Version [3]uint16

// NewVersion creates a version from a string like "Major.Minor.Patch".
//
// Patch is optional.
func NewVersion(ver string) (Version, error) {
	var major, minor, patch uint16
	n, _ := fmt.Sscanf(ver, "%d.%d.%d", &major, &minor, &patch)
	if n < 2 {
		return Version{}, fmt.Errorf("invalid version: %s", ver)
	}
	return Version{major, minor, patch}, nil
}

func (v Version) String() string {
	if v[2] == 0 {
		return fmt.Sprintf("v%d.%d", v[0], v[1])
	}
	return fmt.Sprintf("v%d.%d.%d", v[0], v[1], v[2])
}

// Less returns true if the version is less than another version.
func (v Version) Less(other Version) bool {
	for i, a := range v {
		if a == other[i] {
			continue
		}
		return a < other[i]
	}
	return false
}

// Unspecified returns true if the version is all zero.
func (v Version) Unspecified() bool {
	return v[0] == 0 && v[1] == 0 && v[2] == 0
}
