//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

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

import (
	"golang.org/x/sys/unix"
)

// Acquire creates an exclusive lock on a file for the duration of the process, or until Release(d).
// This method is reentrant.
func Acquire(path string) (int, error) {
	lock, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC, 0600)
	if err != nil {
		return -1, err
	}
	return lock, unix.Flock(lock, unix.LOCK_EX)
}

// AcquireShared creates a shared lock on a file for the duration of the process, or until Release(d).
// This method is reentrant.
func AcquireShared(path string) (int, error) {
	lock, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR, 0600)
	if err != nil {
		return -1, err
	}
	return lock, unix.Flock(lock, unix.LOCK_SH)
}

// Release removes an existing lock held by this process.
func Release(lock int) error {
	return unix.Flock(lock, unix.LOCK_UN)
}
