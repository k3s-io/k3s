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

/*
   Copyright The runc Authors.

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

package seccomp

import (
	"bufio"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// IsEnabled returns if the kernel has been configured to support seccomp.
// From https://github.com/opencontainers/runc/blob/v1.0.0-rc91/libcontainer/seccomp/seccomp_linux.go#L86-L102
func IsEnabled() bool {
	// Try to read from /proc/self/status for kernels > 3.8
	s, err := parseStatusFile("/proc/self/status")
	if err != nil {
		// Check if Seccomp is supported, via CONFIG_SECCOMP.
		if err := unix.Prctl(unix.PR_GET_SECCOMP, 0, 0, 0, 0); err != unix.EINVAL {
			// Make sure the kernel has CONFIG_SECCOMP_FILTER.
			if err := unix.Prctl(unix.PR_SET_SECCOMP, unix.SECCOMP_MODE_FILTER, 0, 0, 0); err != unix.EINVAL {
				return true
			}
		}
		return false
	}
	_, ok := s["Seccomp"]
	return ok
}

// parseStatusFile is from https://github.com/opencontainers/runc/blob/v1.0.0-rc91/libcontainer/seccomp/seccomp_linux.go#L243-L268
func parseStatusFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	status := make(map[string]string)

	for s.Scan() {
		text := s.Text()
		parts := strings.Split(text, ":")

		if len(parts) <= 1 {
			continue
		}

		status[parts[0]] = parts[1]
	}
	if err := s.Err(); err != nil {
		return nil, err
	}

	return status, nil
}
