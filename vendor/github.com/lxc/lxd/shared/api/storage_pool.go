package api

// StoragePoolsPost represents the fields of a new LXD storage pool
//
// API extension: storage
type StoragePoolsPost struct {
	StoragePoolPut `yaml:",inline"`

	Name   string `json:"name" yaml:"name"`
	Driver string `json:"driver" yaml:"driver"`
}

// StoragePool represents the fields of a LXD storage pool.
//
// API extension: storage
type StoragePool struct {
	StoragePoolPut `yaml:",inline"`

	Name   string   `json:"name" yaml:"name"`
	Driver string   `json:"driver" yaml:"driver"`
	UsedBy []string `json:"used_by" yaml:"used_by"`

	// API extension: clustering
	Status    string   `json:"status" yaml:"status"`
	Locations []string `json:"locations" yaml:"locations"`
}

// StoragePoolPut represents the modifiable fields of a LXD storage pool.
//
// API extension: storage
type StoragePoolPut struct {
	Config map[string]string `json:"config" yaml:"config"`

	// API extension: entity_description
	Description string `json:"description" yaml:"description"`
}

// Writable converts a full StoragePool struct into a StoragePoolPut struct
// (filters read-only fields).
func (storagePool *StoragePool) Writable() StoragePoolPut {
	return storagePool.StoragePoolPut
}
