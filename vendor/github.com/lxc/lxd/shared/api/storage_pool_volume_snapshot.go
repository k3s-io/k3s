package api

// StorageVolumeSnapshotsPost represents the fields available for a new LXD storage volume snapshot
//
// API extension: storage_api_volume_snapshots
type StorageVolumeSnapshotsPost struct {
	Name string `json:"name" yaml:"name"`
}

// StorageVolumeSnapshotPost represents the fields required to rename/move a LXD storage volume snapshot
//
// API extension: storage_api_volume_snapshots
type StorageVolumeSnapshotPost struct {
	Name string `json:"name" yaml:"name"`
}

// StorageVolumeSnapshot represents a LXD storage volume snapshot
//
// API extension: storage_api_volume_snapshots
type StorageVolumeSnapshot struct {
	Name        string            `json:"name" yaml:"name"`
	Config      map[string]string `json:"config" yaml:"config"`
	Description string            `json:"description" yaml:"description"`
}

// StorageVolumeSnapshotPut represents the modifiable fields of a LXD storage volume
//
// API extension: storage_api_volume_snapshots
type StorageVolumeSnapshotPut struct {
	Description string `json:"description" yaml:"description"`
}
