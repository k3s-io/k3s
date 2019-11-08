package api

// ContainerStatePut represents the modifiable fields of a LXD container's state
type ContainerStatePut struct {
	Action   string `json:"action" yaml:"action"`
	Timeout  int    `json:"timeout" yaml:"timeout"`
	Force    bool   `json:"force" yaml:"force"`
	Stateful bool   `json:"stateful" yaml:"stateful"`
}

// ContainerState represents a LXD container's state
type ContainerState struct {
	Status     string                           `json:"status" yaml:"status"`
	StatusCode StatusCode                       `json:"status_code" yaml:"status_code"`
	Disk       map[string]ContainerStateDisk    `json:"disk" yaml:"disk"`
	Memory     ContainerStateMemory             `json:"memory" yaml:"memory"`
	Network    map[string]ContainerStateNetwork `json:"network" yaml:"network"`
	Pid        int64                            `json:"pid" yaml:"pid"`
	Processes  int64                            `json:"processes" yaml:"processes"`

	// API extension: container_cpu_time
	CPU ContainerStateCPU `json:"cpu" yaml:"cpu"`
}

// ContainerStateDisk represents the disk information section of a LXD container's state
type ContainerStateDisk struct {
	Usage int64 `json:"usage" yaml:"usage"`
}

// ContainerStateCPU represents the cpu information section of a LXD container's state
//
// API extension: container_cpu_time
type ContainerStateCPU struct {
	Usage int64 `json:"usage" yaml:"usage"`
}

// ContainerStateMemory represents the memory information section of a LXD container's state
type ContainerStateMemory struct {
	Usage         int64 `json:"usage" yaml:"usage"`
	UsagePeak     int64 `json:"usage_peak" yaml:"usage_peak"`
	SwapUsage     int64 `json:"swap_usage" yaml:"swap_usage"`
	SwapUsagePeak int64 `json:"swap_usage_peak" yaml:"swap_usage_peak"`
}

// ContainerStateNetwork represents the network information section of a LXD container's state
type ContainerStateNetwork struct {
	Addresses []ContainerStateNetworkAddress `json:"addresses" yaml:"addresses"`
	Counters  ContainerStateNetworkCounters  `json:"counters" yaml:"counters"`
	Hwaddr    string                         `json:"hwaddr" yaml:"hwaddr"`
	HostName  string                         `json:"host_name" yaml:"host_name"`
	Mtu       int                            `json:"mtu" yaml:"mtu"`
	State     string                         `json:"state" yaml:"state"`
	Type      string                         `json:"type" yaml:"type"`
}

// ContainerStateNetworkAddress represents a network address as part of the network section of a LXD container's state
type ContainerStateNetworkAddress struct {
	Family  string `json:"family" yaml:"family"`
	Address string `json:"address" yaml:"address"`
	Netmask string `json:"netmask" yaml:"netmask"`
	Scope   string `json:"scope" yaml:"scope"`
}

// ContainerStateNetworkCounters represents packet counters as part of the network section of a LXD container's state
type ContainerStateNetworkCounters struct {
	BytesReceived   int64 `json:"bytes_received" yaml:"bytes_received"`
	BytesSent       int64 `json:"bytes_sent" yaml:"bytes_sent"`
	PacketsReceived int64 `json:"packets_received" yaml:"packets_received"`
	PacketsSent     int64 `json:"packets_sent" yaml:"packets_sent"`
}
