// Copyright 2016 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sysctl

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// Sysctl provides a method to set/get values from /proc/sys - in linux systems
// new interface to set/get values of variables formerly handled by sysctl syscall
// If optional `params` have only one string value - this function will
// set this value into corresponding sysctl variable
func Sysctl(name string, params ...string) (string, error) {
	if len(params) > 1 {
		return "", fmt.Errorf("unexcepted additional parameters")
	} else if len(params) == 1 {
		return setSysctl(name, params[0])
	}
	return getSysctl(name)
}

func getSysctl(name string) (string, error) {
	fullName := filepath.Join("/proc/sys", toNormalName(name))
	fullName = filepath.Clean(fullName)
	data, err := ioutil.ReadFile(fullName)
	if err != nil {
		return "", err
	}

	return string(data[:len(data)-1]), nil
}

func setSysctl(name, value string) (string, error) {
	fullName := filepath.Join("/proc/sys", toNormalName(name))
	fullName = filepath.Clean(fullName)
	if err := ioutil.WriteFile(fullName, []byte(value), 0644); err != nil {
		return "", err
	}

	return getSysctl(name)
}

// Normalize names by using slash as separator
// Sysctl names can use dots or slashes as separator:
// - if dots are used, dots and slashes are interchanged.
// - if slashes are used, slashes and dots are left intact.
// Separator in use is determined by first occurrence.
func toNormalName(name string) string {
	interchange := false
	for _, c := range name {
		if c == '.' {
			interchange = true
			break
		}
		if c == '/' {
			break
		}
	}

	if interchange {
		r := strings.NewReplacer(".", "/", "/", ".")
		return r.Replace(name)
	}
	return name
}
