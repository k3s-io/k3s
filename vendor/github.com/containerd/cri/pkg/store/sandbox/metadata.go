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

package sandbox

import (
	"encoding/json"

	"github.com/pkg/errors"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// NOTE(random-liu):
// 1) Metadata is immutable after created.
// 2) Metadata is checkpointed as containerd container label.

// metadataVersion is current version of sandbox metadata.
const metadataVersion = "v1" // nolint

// versionedMetadata is the internal versioned sandbox metadata.
// nolint
type versionedMetadata struct {
	// Version indicates the version of the versioned sandbox metadata.
	Version string
	// Metadata's type is metadataInternal. If not there will be a recursive call in MarshalJSON.
	Metadata metadataInternal
}

// metadataInternal is for internal use.
type metadataInternal Metadata

// Metadata is the unversioned sandbox metadata.
type Metadata struct {
	// ID is the sandbox id.
	ID string
	// Name is the sandbox name.
	Name string
	// Config is the CRI sandbox config.
	Config *runtime.PodSandboxConfig
	// NetNSPath is the network namespace used by the sandbox.
	NetNSPath string
	// IP of Pod if it is attached to non host network
	IP string
	// RuntimeHandler is the runtime handler name of the pod.
	RuntimeHandler string
}

// MarshalJSON encodes Metadata into bytes in json format.
func (c *Metadata) MarshalJSON() ([]byte, error) {
	return json.Marshal(&versionedMetadata{
		Version:  metadataVersion,
		Metadata: metadataInternal(*c),
	})
}

// UnmarshalJSON decodes Metadata from bytes.
func (c *Metadata) UnmarshalJSON(data []byte) error {
	versioned := &versionedMetadata{}
	if err := json.Unmarshal(data, versioned); err != nil {
		return err
	}
	// Handle old version after upgrade.
	switch versioned.Version {
	case metadataVersion:
		*c = Metadata(versioned.Metadata)
		return nil
	}
	return errors.Errorf("unsupported version: %q", versioned.Version)
}
