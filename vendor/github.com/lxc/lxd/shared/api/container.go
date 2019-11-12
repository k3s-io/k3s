package api

import (
	"time"
)

// ContainersPost represents the fields available for a new LXD container
type ContainersPost struct {
	ContainerPut `yaml:",inline"`

	Name   string          `json:"name" yaml:"name"`
	Source ContainerSource `json:"source" yaml:"source"`

	InstanceType string `json:"instance_type" yaml:"instance_type"`
}

// ContainerPost represents the fields required to rename/move a LXD container
type ContainerPost struct {
	// Used for renames
	Name string `json:"name" yaml:"name"`

	// Used for migration
	Migration bool `json:"migration" yaml:"migration"`

	// API extension: container_stateless_copy
	Live bool `json:"live" yaml:"live"`

	// API extension: container_only_migration
	ContainerOnly bool `json:"container_only" yaml:"container_only"`

	// API extension: container_push_target
	Target *ContainerPostTarget `json:"target" yaml:"target"`
}

// ContainerPostTarget represents the migration target host and operation
//
// API extension: container_push_target
type ContainerPostTarget struct {
	Certificate string            `json:"certificate" yaml:"certificate"`
	Operation   string            `json:"operation,omitempty" yaml:"operation,omitempty"`
	Websockets  map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

// ContainerPut represents the modifiable fields of a LXD container
type ContainerPut struct {
	Architecture string                       `json:"architecture" yaml:"architecture"`
	Config       map[string]string            `json:"config" yaml:"config"`
	Devices      map[string]map[string]string `json:"devices" yaml:"devices"`
	Ephemeral    bool                         `json:"ephemeral" yaml:"ephemeral"`
	Profiles     []string                     `json:"profiles" yaml:"profiles"`

	// For snapshot restore
	Restore  string `json:"restore,omitempty" yaml:"restore,omitempty"`
	Stateful bool   `json:"stateful" yaml:"stateful"`

	// API extension: entity_description
	Description string `json:"description" yaml:"description"`
}

// Container represents a LXD container
type Container struct {
	ContainerPut `yaml:",inline"`

	CreatedAt       time.Time                    `json:"created_at" yaml:"created_at"`
	ExpandedConfig  map[string]string            `json:"expanded_config" yaml:"expanded_config"`
	ExpandedDevices map[string]map[string]string `json:"expanded_devices" yaml:"expanded_devices"`
	Name            string                       `json:"name" yaml:"name"`
	Status          string                       `json:"status" yaml:"status"`
	StatusCode      StatusCode                   `json:"status_code" yaml:"status_code"`

	// API extension: container_last_used_at
	LastUsedAt time.Time `json:"last_used_at" yaml:"last_used_at"`

	// API extension: clustering
	Location string `json:"location" yaml:"location"`
}

// ContainerFull is a combination of Container, ContainerState and CotnainerSnapshot
//
// API extension: container_full
type ContainerFull struct {
	Container `yaml:",inline"`

	Backups   []ContainerBackup   `json:"backups" yaml:"backups"`
	State     *ContainerState     `json:"state" yaml:"state"`
	Snapshots []ContainerSnapshot `json:"snapshots" yaml:"snapshots"`
}

// Writable converts a full Container struct into a ContainerPut struct (filters read-only fields)
func (c *Container) Writable() ContainerPut {
	return c.ContainerPut
}

// IsActive checks whether the container state indicates the container is active
func (c Container) IsActive() bool {
	switch c.StatusCode {
	case Stopped:
		return false
	case Error:
		return false
	default:
		return true
	}
}

// ContainerSource represents the creation source for a new container
type ContainerSource struct {
	Type        string `json:"type" yaml:"type"`
	Certificate string `json:"certificate" yaml:"certificate"`

	// For "image" type
	Alias       string            `json:"alias,omitempty" yaml:"alias,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
	Properties  map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`
	Server      string            `json:"server,omitempty" yaml:"server,omitempty"`
	Secret      string            `json:"secret,omitempty" yaml:"secret,omitempty"`
	Protocol    string            `json:"protocol,omitempty" yaml:"protocol,omitempty"`

	// For "migration" and "copy" types
	BaseImage string `json:"base-image,omitempty" yaml:"base-image,omitempty"`

	// For "migration" type
	Mode       string            `json:"mode,omitempty" yaml:"mode,omitempty"`
	Operation  string            `json:"operation,omitempty" yaml:"operation,omitempty"`
	Websockets map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`

	// For "copy" type
	Source string `json:"source,omitempty" yaml:"source,omitempty"`

	// API extension: container_push
	Live bool `json:"live,omitempty" yaml:"live,omitempty"`

	// API extension: container_only_migration
	ContainerOnly bool `json:"container_only,omitempty" yaml:"container_only,omitempty"`

	// API extension: container_incremental_copy
	Refresh bool `json:"refresh,omitempty" yaml:"refresh,omitempty"`

	// API extension: container_copy_project
	Project string `json:"project,omitempty" yaml:"project,omitempty"`
}
