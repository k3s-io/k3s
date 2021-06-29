// +build windows

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

package crictl

import (
	"os"
	"path/filepath"
)

var defaultRuntimeEndpoints = []string{"npipe:////./pipe/dockershim", "npipe:////./pipe/containerd", "npipe:////./pipe/crio"}
var defaultConfigPath string

func init() {
	defaultConfigPath = filepath.Join(os.Getenv("USERPROFILE"), ".crictl", "crictl.yaml")
}
