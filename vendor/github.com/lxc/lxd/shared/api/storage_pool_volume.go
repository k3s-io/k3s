package api

// StorageVolumesPost represents the fields of a new LXD storage pool volume
//
// API extension: storage
type StorageVolumesPost struct {
	StorageVolumePut `yaml:",inline"`

	Name string `json:"name" yaml:"name"`
	Type string `json:"type" yaml:"type"`

	// API extension: storage_api_local_volume_handling
	Source StorageVolumeSource `json:"source" yaml:"source"`
}

// StorageVolumePost represents the fields required to rename a LXD storage pool volume
//
// API extension: storage_api_volume_rename
type StorageVolumePost struct {
	Name string `json:"name" yaml:"name"`

	// API extension: storage_api_local_volume_handling
	Pool string `json:"pool,omitempty" yaml:"pool,omitempty"`

	// API extension: storage_api_remote_volume_handling
	Migration bool `json:"migration" yaml:"migration"`

	// API extension: storage_api_remote_volume_handling
	Target *StorageVolumePostTarget `json:"target" yaml:"target"`

	// API extension: storage_api_remote_volume_snapshots
	VolumeOnly bool `json:"volume_only" yaml:"volume_only"`
}

// StorageVolumePostTarget represents the migration target host and operation
//
// API extension: storage_api_remote_volume_handling
type StorageVolumePostTarget struct {
	Certificate string            `json:"certificate" yaml:"certificate"`
	Operation   string            `json:"operation,omitempty" yaml:"operation,omitempty"`
	Websockets  map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

// StorageVolume represents the fields of a LXD storage volume.
//
// API extension: storage
type StorageVolume struct {
	StorageVolumePut `yaml:",inline"`
	Name             string   `json:"name" yaml:"name"`
	Type             string   `json:"type" yaml:"type"`
	UsedBy           []string `json:"used_by" yaml:"used_by"`

	// API extension: clustering
	Location string `json:"location" yaml:"location"`
}

// StorageVolumePut represents the modifiable fields of a LXD storage volume.
//
// API extension: storage
type StorageVolumePut struct {
	Config map[string]string `json:"config" yaml:"config"`

	// API extension: entity_description
	Description string `json:"description" yaml:"description"`

	// API extension: storage_api_volume_snapshots
	Restore string `json:"restore,omitempty" yaml:"restore,omitempty"`
}

// StorageVolumeSource represents the creation source for a new storage volume.
//
// API extension: storage_api_local_volume_handling
type StorageVolumeSource struct {
	Name string `json:"name" yaml:"name"`
	Type string `json:"type" yaml:"type"`
	Pool string `json:"pool" yaml:"pool"`

	// API extension: storage_api_remote_volume_handling
	Certificate string            `json:"certificate" yaml:"certificate"`
	Mode        string            `json:"mode,omitempty" yaml:"mode,omitempty"`
	Operation   string            `json:"operation,omitempty" yaml:"operation,omitempty"`
	Websockets  map[string]string `json:"secrets,omitempty" yaml:"secrets,omitempty"`

	// API extension: storage_api_volume_snapshots
	VolumeOnly bool `json:"volume_only" yaml:"volume_only"`
}

// Writable converts a full StorageVolume struct into a StorageVolumePut struct
// (filters read-only fields).
func (storageVolume *StorageVolume) Writable() StorageVolumePut {
	return storageVolume.StorageVolumePut
}
