//+build !linux

package genetlink

import (
	"fmt"
	"runtime"
)

// errUnimplemented is returned by all functions on platforms that
// cannot make use of generic netlink.
var errUnimplemented = fmt.Errorf("generic netlink not implemented on %s/%s",
	runtime.GOOS, runtime.GOARCH)

// getFamily always returns an error.
func (c *Conn) getFamily(name string) (Family, error) {
	return Family{}, errUnimplemented
}

// listFamilies always returns an error.
func (c *Conn) listFamilies() ([]Family, error) {
	return nil, errUnimplemented
}
