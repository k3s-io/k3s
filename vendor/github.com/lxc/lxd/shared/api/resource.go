package api

// Resources represents the system resources avaible for LXD
// API extension: resources
type Resources struct {
	CPU    ResourcesCPU    `json:"cpu" yaml:"cpu"`
	Memory ResourcesMemory `json:"memory" yaml:"memory"`

	// API extension: resources_gpu
	GPU ResourcesGPU `json:"gpu" yaml:"gpu"`

	// API extension: resources_v2
	Network ResourcesNetwork `json:"network" yaml:"network"`
	Storage ResourcesStorage `json:"storage" yaml:"storage"`
}

// ResourcesCPU represents the cpu resources available on the system
// API extension: resources
type ResourcesCPU struct {
	// API extension: resources_v2
	Architecture string `json:"architecture" yaml:"architecture"`

	Sockets []ResourcesCPUSocket `json:"sockets" yaml:"sockets"`
	Total   uint64               `json:"total" yaml:"total"`
}

// ResourcesCPUSocket represents a CPU socket on the system
// API extension: resources_v2
type ResourcesCPUSocket struct {
	Name   string `json:"name,omitempty" yaml:"name,omitempty"`
	Vendor string `json:"vendor,omitempty" yaml:"vendor,omitempty"`

	Socket uint64              `json:"socket" yaml:"socket"`
	Cache  []ResourcesCPUCache `json:"cache,omitempty" yaml:"cache,omitempty"`
	Cores  []ResourcesCPUCore  `json:"cores" yaml:"cores"`

	Frequency        uint64 `json:"frequency,omitempty" yaml:"frequency,omitempty"`
	FrequencyMinimum uint64 `json:"frequency_minimum,omitempty" yaml:"frequency_minimum,omitempty"`
	FrequencyTurbo   uint64 `json:"frequency_turbo,omitempty" yaml:"frequency_turbo,omitempty"`
}

// ResourcesCPUCache represents a CPU cache
// API extension: resources_v2
type ResourcesCPUCache struct {
	Level uint64 `json:"level" yaml:"level"`
	Type  string `json:"type" yaml:"type"`
	Size  uint64 `json:"size" yaml:"size"`
}

// ResourcesCPUCore represents a CPU core on the system
// API extension: resources_v2
type ResourcesCPUCore struct {
	Core     uint64 `json:"core" yaml:"core"`
	NUMANode uint64 `json:"numa_node" yaml:"numa_node"`

	Threads []ResourcesCPUThread `json:"threads" yaml:"threads"`

	Frequency uint64 `json:"frequency,omitempty" yaml:"frequency,omitempty"`
}

// ResourcesCPUThread represents a CPU thread on the system
// API extension: resources_v2
type ResourcesCPUThread struct {
	ID     int64  `json:"id" yaml:"id"`
	Thread uint64 `json:"thread" yaml:"thread"`
	Online bool   `json:"online" yaml:"online"`
}

// ResourcesGPU represents the GPU resources available on the system
// API extension: resources_gpu
type ResourcesGPU struct {
	Cards []ResourcesGPUCard `json:"cards" yaml:"cards"`
	Total uint64             `json:"total" yaml:"total"`
}

// ResourcesGPUCard represents a GPU card on the system
// API extension: resources_v2
type ResourcesGPUCard struct {
	Driver        string `json:"driver,omitempty" yaml:"driver,omitempty"`
	DriverVersion string `json:"driver_version,omitempty" yaml:"driver_version,omitempty"`

	DRM    *ResourcesGPUCardDRM    `json:"drm,omitempty" yaml:"drm,omitempty"`
	SRIOV  *ResourcesGPUCardSRIOV  `json:"sriov,omitempty" yaml:"sriov,omitempty"`
	Nvidia *ResourcesGPUCardNvidia `json:"nvidia,omitempty" yaml:"nvidia,omitempty"`

	NUMANode   uint64 `json:"numa_node" yaml:"numa_node"`
	PCIAddress string `json:"pci_address,omitempty" yaml:"pci_address,omitempty"`

	Vendor    string `json:"vendor,omitempty" yaml:"vendor,omitempty"`
	VendorID  string `json:"vendor_id,omitempty" yaml:"vendor_id,omitempty"`
	Product   string `json:"product,omitempty" yaml:"product,omitempty"`
	ProductID string `json:"product_id,omitempty" yaml:"product_id,omitempty"`
}

// ResourcesGPUCardDRM represents the Linux DRM configuration of the GPU
// API extension: resources_v2
type ResourcesGPUCardDRM struct {
	ID uint64 `json:"id" yaml:"id"`

	CardName   string `json:"card_name" yaml:"card_name"`
	CardDevice string `json:"card_device" yaml:"card_device"`

	ControlName   string `json:"control_name,omitempty" yaml:"control_name,omitempty"`
	ControlDevice string `json:"control_device,omitempty" yaml:"control_device,omitempty"`

	RenderName   string `json:"render_name,omitempty" yaml:"render_name,omitempty"`
	RenderDevice string `json:"render_device,omitempty" yaml:"render_device,omitempty"`
}

// ResourcesGPUCardSRIOV represents the SRIOV configuration of the GPU
// API extension: resources_v2
type ResourcesGPUCardSRIOV struct {
	CurrentVFs uint64 `json:"current_vfs" yaml:"current_vfs"`
	MaximumVFs uint64 `json:"maximum_vfs" yaml:"maximum_vfs"`

	VFs []ResourcesGPUCard `json:"vfs" yaml:"vfs"`
}

// ResourcesGPUCardNvidia represents additional information for NVIDIA GPUs
// API extension: resources_gpu
type ResourcesGPUCardNvidia struct {
	CUDAVersion string `json:"cuda_version,omitempty" yaml:"cuda_version,omitempty"`
	NVRMVersion string `json:"nvrm_version,omitempty" yaml:"nvrm_version,omitempty"`

	Brand        string `json:"brand" yaml:"brand"`
	Model        string `json:"model" yaml:"model"`
	UUID         string `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Architecture string `json:"architecture,omitempty" yaml:"architecture,omitempty"`

	// API extension: resources_v2
	CardName   string `json:"card_name" yaml:"card_name"`
	CardDevice string `json:"card_device" yaml:"card_device"`
}

// ResourcesNetwork represents the network cards available on the system
// API extension: resources_v2
type ResourcesNetwork struct {
	Cards []ResourcesNetworkCard `json:"cards" yaml:"cards"`
	Total uint64                 `json:"total" yaml:"total"`
}

// ResourcesNetworkCard represents a network card on the system
// API extension: resources_v2
type ResourcesNetworkCard struct {
	Driver        string `json:"driver,omitempty" yaml:"driver,omitempty"`
	DriverVersion string `json:"driver_version,omitempty" yaml:"driver_version,omitempty"`

	Ports []ResourcesNetworkCardPort `json:"ports,omitempty" yaml:"ports,omitempty"`
	SRIOV *ResourcesNetworkCardSRIOV `json:"sriov,omitempty" yaml:"sriov,omitempty"`

	NUMANode   uint64 `json:"numa_node" yaml:"numa_node"`
	PCIAddress string `json:"pci_address,omitempty" yaml:"pci_address,omitempty"`

	Vendor    string `json:"vendor,omitempty" yaml:"vendor,omitempty"`
	VendorID  string `json:"vendor_id,omitempty" yaml:"vendor_id,omitempty"`
	Product   string `json:"product,omitempty" yaml:"product,omitempty"`
	ProductID string `json:"product_id,omitempty" yaml:"product_id,omitempty"`

	// API extension: resources_network_firmware
	FirmwareVersion string `json:"firmware_version,omitempty" yaml:"firmware_version,omitempty"`
}

// ResourcesNetworkCardPort represents a network port on the system
// API extension: resources_v2
type ResourcesNetworkCardPort struct {
	ID       string `json:"id" yaml:"id"`
	Address  string `json:"address,omitempty" yaml:"address,omitempty"`
	Port     uint64 `json:"port" yaml:"port"`
	Protocol string `json:"protocol" yaml:"protocol"`

	SupportedModes []string `json:"supported_modes,omitempty" yaml:"supported_modes,omitempty"`
	SupportedPorts []string `json:"supported_ports,omitempty" yaml:"supported_ports,omitempty"`

	PortType        string `json:"port_type,omitempty" yaml:"port_type,omitempty"`
	TransceiverType string `json:"transceiver_type,omitempty" yaml:"transceiver_type,omitempty"`

	AutoNegotiation bool   `json:"auto_negotiation" yaml:"auto_negotiation"`
	LinkDetected    bool   `json:"link_detected" yaml:"link_detected"`
	LinkSpeed       uint64 `json:"link_speed,omitempty" yaml:"link_speed,omitempty"`
	LinkDuplex      string `json:"link_duplex,omitempty" yaml:"link_duplex,omitempty"`

	// API extension: resources_infiniband
	Infiniband *ResourcesNetworkCardPortInfiniband `json:"infiniband,omitempty" yaml:"infiniband,omitempty"`
}

// ResourcesNetworkCardPortInfiniband represents the Linux Infiniband configuration for the port
// API extension: resources_infiniband
type ResourcesNetworkCardPortInfiniband struct {
	IsSMName   string `json:"issm_name,omitempty" yaml:"issm_name,omitempty"`
	IsSMDevice string `json:"issm_device,omitempty" yaml:"issm_device,omitempty"`

	MADName   string `json:"mad_name,omitempty" yaml:"mad_name,omitempty"`
	MADDevice string `json:"mad_device,omitempty" yaml:"mad_device,omitempty"`

	VerbName   string `json:"verb_name,omitempty" yaml:"verb_name,omitempty"`
	VerbDevice string `json:"verb_device,omitempty" yaml:"verb_device,omitempty"`
}

// ResourcesNetworkCardSRIOV represents the SRIOV configuration of the network card
// API extension: resources_v2
type ResourcesNetworkCardSRIOV struct {
	CurrentVFs uint64 `json:"current_vfs" yaml:"current_vfs"`
	MaximumVFs uint64 `json:"maximum_vfs" yaml:"maximum_vfs"`

	VFs []ResourcesNetworkCard `json:"vfs" yaml:"vfs"`
}

// ResourcesStorage represents the local storage
// API extension: resources_v2
type ResourcesStorage struct {
	Disks []ResourcesStorageDisk `json:"disks" yaml:"disks"`
	Total uint64                 `json:"total" yaml:"total"`
}

// ResourcesStorageDisk represents a disk
// API extension: resources_v2
type ResourcesStorageDisk struct {
	ID       string `json:"id" yaml:"id"`
	Device   string `json:"device" yaml:"device"`
	Model    string `json:"model,omitempty" yaml:"model,omitempty"`
	Type     string `json:"type,omitempty" yaml:"type,omitempty"`
	ReadOnly bool   `json:"read_only" yaml:"read_only"`
	Size     uint64 `json:"size" yaml:"size"`

	Removable bool   `json:"removable" yaml:"removable"`
	WWN       string `json:"wwn,omitempty" yaml:"wwn,omitempty"`
	NUMANode  uint64 `json:"numa_node" yaml:"numa_node"`

	// API extension: resources_disk_sata
	DevicePath      string `json:"device_path" yaml:"device_path"`
	BlockSize       uint64 `json:"block_size" yaml:"block_size"`
	FirmwareVersion string `json:"firmware_version,omitempty" yaml:"firmware_version,omitempty"`
	RPM             uint64 `json:"rpm" yaml:"rpm"`
	Serial          string `json:"serial,omitempty" yaml:"serial,omitempty"`

	Partitions []ResourcesStorageDiskPartition `json:"partitions" yaml:"partitions"`
}

// ResourcesStorageDiskPartition represents a partition on a disk
// API extension: resources_v2
type ResourcesStorageDiskPartition struct {
	ID       string `json:"id" yaml:"id"`
	Device   string `json:"device" yaml:"device"`
	ReadOnly bool   `json:"read_only" yaml:"read_only"`
	Size     uint64 `json:"size" yaml:"size"`

	Partition uint64 `json:"partition" yaml:"partition"`
}

// ResourcesMemory represents the memory resources available on the system
// API extension: resources
type ResourcesMemory struct {
	// API extension: resources_v2
	Nodes          []ResourcesMemoryNode `json:"nodes,omitempty" yaml:"nodes,omitempty"`
	HugepagesTotal uint64                `json:"hugepages_total" yaml:"hugepages_total"`
	HugepagesUsed  uint64                `json:"hugepages_used" yaml:"hugepages_used"`
	HugepagesSize  uint64                `json:"hugepages_size" yaml:"hugepages_size"`

	Used  uint64 `json:"used" yaml:"used"`
	Total uint64 `json:"total" yaml:"total"`
}

// ResourcesMemoryNode represents the node-specific memory resources available on the system
// API extension: resources_v2
type ResourcesMemoryNode struct {
	NUMANode       uint64 `json:"numa_node" yaml:"numa_node"`
	HugepagesUsed  uint64 `json:"hugepages_used" yaml:"hugepages_used"`
	HugepagesTotal uint64 `json:"hugepages_total" yaml:"hugepages_total"`

	Used  uint64 `json:"used" yaml:"used"`
	Total uint64 `json:"total" yaml:"total"`
}

// ResourcesStoragePool represents the resources available to a given storage pool
// API extension: resources
type ResourcesStoragePool struct {
	Space  ResourcesStoragePoolSpace  `json:"space,omitempty" yaml:"space,omitempty"`
	Inodes ResourcesStoragePoolInodes `json:"inodes,omitempty" yaml:"inodes,omitempty"`
}

// ResourcesStoragePoolSpace represents the space available to a given storage pool
// API extension: resources
type ResourcesStoragePoolSpace struct {
	Used  uint64 `json:"used,omitempty" yaml:"used,omitempty"`
	Total uint64 `json:"total" yaml:"total"`
}

// ResourcesStoragePoolInodes represents the inodes available to a given storage pool
// API extension: resources
type ResourcesStoragePoolInodes struct {
	Used  uint64 `json:"used" yaml:"used"`
	Total uint64 `json:"total" yaml:"total"`
}
