package api

import "time"

// ContainerBackupsPost represents the fields available for a new LXD container backup
// API extension: container_backup
type ContainerBackupsPost struct {
	Name             string    `json:"name" yaml:"name"`
	ExpiresAt        time.Time `json:"expires_at" yaml:"expires_at"`
	ContainerOnly    bool      `json:"container_only" yaml:"container_only"`
	OptimizedStorage bool      `json:"optimized_storage" yaml:"optimized_storage"`
}

// ContainerBackup represents a LXD container backup
// API extension: container_backup
type ContainerBackup struct {
	Name             string    `json:"name" yaml:"name"`
	CreatedAt        time.Time `json:"created_at" yaml:"created_at"`
	ExpiresAt        time.Time `json:"expires_at" yaml:"expires_at"`
	ContainerOnly    bool      `json:"container_only" yaml:"container_only"`
	OptimizedStorage bool      `json:"optimized_storage" yaml:"optimized_storage"`
}

// ContainerBackupPost represents the fields available for the renaming of a
// container backup
// API extension: container_backup
type ContainerBackupPost struct {
	Name string `json:"name" yaml:"name"`
}
