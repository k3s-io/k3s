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

package store

import "github.com/containerd/containerd/errdefs"

var (
	// ErrAlreadyExist is the error returned when data added in the store
	// already exists.
	//
	// This error has been DEPRECATED and will be removed in 1.5. Please switch
	// usage directly to `errdefs.ErrAlreadyExists`.
	ErrAlreadyExist = errdefs.ErrAlreadyExists
	// ErrNotExist is the error returned when data is not in the store.
	//
	// This error has been DEPRECATED and will be removed in 1.5. Please switch
	// usage directly to `errdefs.ErrNotFound`.
	ErrNotExist = errdefs.ErrNotFound
)
