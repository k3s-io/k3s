//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly
// +build !linux,!darwin,!freebsd,!openbsd,!netbsd,!dragonfly

/*
Copyright 2016 The Kubernetes Authors.

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

package flock

// Acquire is not implemented on non-unix systems.
func Acquire(path string) (int, error) {
	return -1, nil
}

// AcquireShared creates a shared lock on a file for the duration of the process, or until Release(d).
// This method is reentrant.
func AcquireShared(path string) (int, error) {
	return 0, nil
}

// Release is not implemented on non-unix systems.
func Release(lock int) error {
	return nil
}

// CheckLock checks whether any process is using the lock
func CheckLock(path string) bool {
	return false
}
