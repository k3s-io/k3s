/*
Copyright 2017 The Kubernetes Authors.

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

package container

import "sync"

// WithFakeStatus adds fake status to the container.
func WithFakeStatus(status Status) Opts {
	return func(c *Container) error {
		c.Status = &fakeStatusStorage{status: status}
		if status.FinishedAt != 0 {
			// Fake the TaskExit event
			c.Stop()
		}
		return nil
	}
}

// fakeStatusStorage is a fake status storage for testing.
type fakeStatusStorage struct {
	sync.RWMutex
	status Status
}

func (f *fakeStatusStorage) Get() Status {
	f.RLock()
	defer f.RUnlock()
	return f.status
}

func (f *fakeStatusStorage) UpdateSync(u UpdateFunc) error {
	return f.Update(u)
}

func (f *fakeStatusStorage) Update(u UpdateFunc) error {
	f.Lock()
	defer f.Unlock()
	newStatus, err := u(f.status)
	if err != nil {
		return err
	}
	f.status = newStatus
	return nil
}

func (f *fakeStatusStorage) Delete() error {
	return nil
}
