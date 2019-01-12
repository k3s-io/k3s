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

package constants

// TODO(random-liu): Merge annotations package into this package.

const (
	// K8sContainerdNamespace is the namespace we use to connect containerd.
	K8sContainerdNamespace = "k8s.io"
	// CRIVersion is the CRI version supported by the CRI plugin.
	CRIVersion = "v1alpha2"
)
