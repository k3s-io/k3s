/*
Copyright 2018 The Containerd Authors.

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

package image

import "github.com/pkg/errors"

// NewFakeStore returns an image store with predefined images.
// Update is not allowed for this fake store.
func NewFakeStore(images []Image) (*Store, error) {
	s := NewStore(nil)
	for _, i := range images {
		for _, ref := range i.References {
			s.refCache[ref] = i.ID
		}
		if err := s.store.add(i); err != nil {
			return nil, errors.Wrapf(err, "add image %+v", i)
		}
	}
	return s, nil
}
