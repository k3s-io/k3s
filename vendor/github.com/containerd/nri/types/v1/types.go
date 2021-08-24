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

package v1

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// Plugin type and configuration
type Plugin struct {
	// Type of plugin
	Type string `json:"type"`
	// Conf for the specific plugin
	Conf json.RawMessage `json:"conf,omitempty"`
}

// ConfigList for the global configuration of NRI
//
// Normally located at /etc/nri/conf.json
type ConfigList struct {
	// Verion of the list
	Version string `json:"version"`
	// Plugins
	Plugins []*Plugin `json:"plugins"`
}

// Spec for the container being processed
type Spec struct {
	// Resources struct from the OCI specification
	//
	// Can be WindowsResources or LinuxResources
	Resources json.RawMessage `json:"resources"`
	// Namespaces for the container
	Namespaces map[string]string `json:"namespaces,omitempty"`
	// CgroupsPath for the container
	CgroupsPath string `json:"cgroupsPath,omitempty"`
	// Annotations passed down to the OCI runtime specification
	Annotations map[string]string `json:"annotations,omitempty"`
}

// State of the request
type State string

const (
	// Create the initial resource for the container
	Create State = "create"
	// Delete any resources for the container
	Delete State = "delete"
	// Update the resources for the container
	Update State = "update"
	// Pause action of the container
	Pause State = "pause"
	// Resume action for the container
	Resume State = "resume"
)

// Request for a plugin invocation
type Request struct {
	// Conf specific for the plugin
	Conf json.RawMessage `json:"conf,omitempty"`

	// Version of the plugin
	Version string `json:"version"`
	// State action for the request
	State State `json:"state"`
	// ID for the container
	ID string `json:"id"`
	// SandboxID for the sandbox that the request belongs to
	//
	// If ID and SandboxID are the same, this is a request for the sandbox
	// SandboxID is empty for a non sandboxed container
	SandboxID string `json:"sandboxID,omitempty"`
	// Pid of the container
	//
	// -1 if there is no pid
	Pid int `json:"pid,omitempty"`
	// Spec generated from the OCI runtime specification
	Spec *Spec `json:"spec"`
	// Labels of a sandbox
	Labels map[string]string `json:"labels,omitempty"`
	// Results from previous plugins in the chain
	Results []*Result `json:"results,omitempty"`
}

// IsSandbox returns true if the request is for a sandbox
func (r *Request) IsSandbox() bool {
	return r.ID == r.SandboxID
}

// NewResult returns a result from the original request
func (r *Request) NewResult(plugin string) *Result {
	return &Result{
		Plugin:   plugin,
		Version:  r.Version,
		Metadata: make(map[string]string),
	}
}

// Result of the plugin invocation
type Result struct {
	// Plugin name that populated the result
	Plugin string `json:"plugin"`
	// Version of the plugin
	Version string `json:"version"`
	// Error message in case of failures
	Error string `json:"error"`
	// Metadata specific to actions taken by the plugin
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Err creates an Error object if ErrorMessage is populated
func (r *Result) Err() error {
	if r.Error != "" {
		return errors.New(r.Error)
	}
	return nil
}
