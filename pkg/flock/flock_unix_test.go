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
	"os/exec"
	"strings"
	"testing"
)

// checkLock checks whether any process is using the lock
func checkLock(path string) bool {
	lockByte, _ := exec.Command("lsof", "-w", "-F", "lfn", path).Output()
	locks := string(lockByte)
	if locks == "" {
		return false
	}
	readWriteLock := strings.Split(locks, "\n")[2]
	return readWriteLock == "lR" || readWriteLock == "lW"
}

func Test_UnitFlock(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantCheck bool
		wantErr   bool
	}{
		{
			name: "Basic Flock Test",
			path: "/tmp/testlock.test",

			wantCheck: true,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock, err := Acquire(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Acquire() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got := checkLock(tt.path); got != tt.wantCheck {
				t.Errorf("CheckLock() = %+v\nWant = %+v", got, tt.wantCheck)
			}

			if err := Release(lock); (err != nil) != tt.wantErr {
				t.Errorf("Release() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got := checkLock(tt.path); got == tt.wantCheck {
				t.Errorf("CheckLock() = %+v\nWant = %+v", got, !tt.wantCheck)
			}
		})
	}
}
