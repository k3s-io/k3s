// +build !windows,!linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package netns

import (
	"github.com/pkg/errors"
)

var errNotImplementedOnUnix = errors.New("not implemented on unix")

// NetNS holds network namespace.
type NetNS struct {
	path string
}

// NewNetNS creates a network namespace.
func NewNetNS(baseDir string) (*NetNS, error) {
	return nil, errNotImplementedOnUnix
}

// LoadNetNS loads existing network namespace.
func LoadNetNS(path string) *NetNS {
	return &NetNS{path: path}
}

// Remove removes network namespace. Remove is idempotent, meaning it might
// be invoked multiple times and provides consistent result.
func (n *NetNS) Remove() error {
	return errNotImplementedOnUnix
}

// Closed checks whether the network namespace has been closed.
func (n *NetNS) Closed() (bool, error) {
	return false, errNotImplementedOnUnix
}

// GetPath returns network namespace path for sandbox container
func (n *NetNS) GetPath() string {
	return n.path
}

// NOTE: Do function is not supported.
