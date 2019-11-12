package api

import (
	"time"
)

// InstanceType represents the type if instance being returned or requested via the API.
type InstanceType string

// InstanceTypeAny defines the instance type value for requesting any instance type.
const InstanceTypeAny = InstanceType("")

// InstanceTypeContainer defines the instance type value for a container.
const InstanceTypeContainer = InstanceType("container")

// InstanceTypeVM defines the instance type value for a virtual-machine.
const InstanceTypeVM = InstanceType("virtual-machine")

// InstancesPost represents the fields available for a new LXD instance.
//
// API extension: instances
type InstancesPost struct {
	InstancePut `yaml:",inline"`

	Name         string         `json:"name" yaml:"name"`
	Source       InstanceSource `json:"source" yaml:"source"`
	InstanceType string         `json:"instance_type" yaml:"instance_type"`
	Type         InstanceType   `json:"type" yaml:"type"`
}

// InstancePost represents the fields required to rename/move a LXD instance.
//
// API extension: instances
type InstancePost struct {
	Name          string              `json:"name" yaml:"name"`
	Migration     bool                `json:"migration" yaml:"migration"`
	Live          bool                `json:"live" yaml:"live"`
	InstanceOnly  bool                `json:"instance_only" yaml:"instance_only"`
	ContainerOnly bool                `json:"container_only" yaml:"container_only"` // Deprecated, use InstanceOnly.
	Target        *InstancePostTarget `json:"target" yaml:"target"`
}

// InstancePostTarget represents the migration target host and operation.
//
// API extension: instances
type InstancePostTarget struct {
	Certificate string            `json:"certificate" yaml:"certificate"`
	Operation   string            `json:"operation,omitempty" yaml:"operation,omitempty"`
	Websockets  map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

// InstancePut represents the modifiable fields of a LXD instance.
//
// API extension: instances
type InstancePut struct {
	Architecture string                       `json:"architecture" yaml:"architecture"`
	Config       map[string]string            `json:"config" yaml:"config"`
	Devices      map[string]map[string]string `json:"devices" yaml:"devices"`
	Ephemeral    bool                         `json:"ephemeral" yaml:"ephemeral"`
	Profiles     []string                     `json:"profiles" yaml:"profiles"`
	Restore      string                       `json:"restore,omitempty" yaml:"restore,omitempty"`
	Stateful     bool                         `json:"stateful" yaml:"stateful"`
	Description  string                       `json:"description" yaml:"description"`
}

// Instance represents a LXD instance.
//
// API extension: instances
type Instance struct {
	InstancePut `yaml:",inline"`

	CreatedAt       time.Time                    `json:"created_at" yaml:"created_at"`
	ExpandedConfig  map[string]string            `json:"expanded_config" yaml:"expanded_config"`
	ExpandedDevices map[string]map[string]string `json:"expanded_devices" yaml:"expanded_devices"`
	Name            string                       `json:"name" yaml:"name"`
	Status          string                       `json:"status" yaml:"status"`
	StatusCode      StatusCode                   `json:"status_code" yaml:"status_code"`
	LastUsedAt      time.Time                    `json:"last_used_at" yaml:"last_used_at"`
	Location        string                       `json:"location" yaml:"location"`
	Type            string                       `json:"type" yaml:"type"`
}

// InstanceFull is a combination of Instance, InstanceBackup, InstanceState and InstanceSnapshot.
//
// API extension: instances
type InstanceFull struct {
	Instance `yaml:",inline"`

	Backups   []InstanceBackup   `json:"backups" yaml:"backups"`
	State     *InstanceState     `json:"state" yaml:"state"`
	Snapshots []InstanceSnapshot `json:"snapshots" yaml:"snapshots"`
}

// Writable converts a full Instance struct into a InstancePut struct (filters read-only fields).
//
// API extension: instances
func (c *Instance) Writable() InstancePut {
	return c.InstancePut
}

// IsActive checks whether the instance state indicates the instance is active.
//
// API extension: instances
func (c Instance) IsActive() bool {
	switch c.StatusCode {
	case Stopped:
		return false
	case Error:
		return false
	default:
		return true
	}
}

// InstanceSource represents the creation source for a new instance.
//
// API extension: instances
type InstanceSource struct {
	Type          string            `json:"type" yaml:"type"`
	Certificate   string            `json:"certificate" yaml:"certificate"`
	Alias         string            `json:"alias,omitempty" yaml:"alias,omitempty"`
	Fingerprint   string            `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
	Properties    map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`
	Server        string            `json:"server,omitempty" yaml:"server,omitempty"`
	Secret        string            `json:"secret,omitempty" yaml:"secret,omitempty"`
	Protocol      string            `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	BaseImage     string            `json:"base-image,omitempty" yaml:"base-image,omitempty"`
	Mode          string            `json:"mode,omitempty" yaml:"mode,omitempty"`
	Operation     string            `json:"operation,omitempty" yaml:"operation,omitempty"`
	Websockets    map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	Source        string            `json:"source,omitempty" yaml:"source,omitempty"`
	Live          bool              `json:"live,omitempty" yaml:"live,omitempty"`
	InstanceOnly  bool              `json:"instance_only,omitempty" yaml:"instance_only,omitempty"`
	ContainerOnly bool              `json:"container_only,omitempty" yaml:"container_only,omitempty"` // Deprecated, use InstanceOnly.
	Refresh       bool              `json:"refresh,omitempty" yaml:"refresh,omitempty"`
	Project       string            `json:"project,omitempty" yaml:"project,omitempty"`
}
