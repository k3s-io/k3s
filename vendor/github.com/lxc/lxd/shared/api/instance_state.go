package api

// InstanceStatePut represents the modifiable fields of a LXD instance's state.
//
// API extension: instances
type InstanceStatePut struct {
	Action   string `json:"action" yaml:"action"`
	Timeout  int    `json:"timeout" yaml:"timeout"`
	Force    bool   `json:"force" yaml:"force"`
	Stateful bool   `json:"stateful" yaml:"stateful"`
}

// InstanceState represents a LXD instance's state.
//
// API extension: instances
type InstanceState struct {
	Status     string                          `json:"status" yaml:"status"`
	StatusCode StatusCode                      `json:"status_code" yaml:"status_code"`
	Disk       map[string]InstanceStateDisk    `json:"disk" yaml:"disk"`
	Memory     InstanceStateMemory             `json:"memory" yaml:"memory"`
	Network    map[string]InstanceStateNetwork `json:"network" yaml:"network"`
	Pid        int64                           `json:"pid" yaml:"pid"`
	Processes  int64                           `json:"processes" yaml:"processes"`
	CPU        InstanceStateCPU                `json:"cpu" yaml:"cpu"`
}

// InstanceStateDisk represents the disk information section of a LXD instance's state.
//
// API extension: instances
type InstanceStateDisk struct {
	Usage int64 `json:"usage" yaml:"usage"`
}

// InstanceStateCPU represents the cpu information section of a LXD instance's state.
//
// API extension: instances
type InstanceStateCPU struct {
	Usage int64 `json:"usage" yaml:"usage"`
}

// InstanceStateMemory represents the memory information section of a LXD instance's state.
//
// API extension: instances
type InstanceStateMemory struct {
	Usage         int64 `json:"usage" yaml:"usage"`
	UsagePeak     int64 `json:"usage_peak" yaml:"usage_peak"`
	SwapUsage     int64 `json:"swap_usage" yaml:"swap_usage"`
	SwapUsagePeak int64 `json:"swap_usage_peak" yaml:"swap_usage_peak"`
}

// InstanceStateNetwork represents the network information section of a LXD instance's state.
//
// API extension: instances
type InstanceStateNetwork struct {
	Addresses []InstanceStateNetworkAddress `json:"addresses" yaml:"addresses"`
	Counters  InstanceStateNetworkCounters  `json:"counters" yaml:"counters"`
	Hwaddr    string                        `json:"hwaddr" yaml:"hwaddr"`
	HostName  string                        `json:"host_name" yaml:"host_name"`
	Mtu       int                           `json:"mtu" yaml:"mtu"`
	State     string                        `json:"state" yaml:"state"`
	Type      string                        `json:"type" yaml:"type"`
}

// InstanceStateNetworkAddress represents a network address as part of the network section of a LXD
// instance's state.
//
// API extension: instances
type InstanceStateNetworkAddress struct {
	Family  string `json:"family" yaml:"family"`
	Address string `json:"address" yaml:"address"`
	Netmask string `json:"netmask" yaml:"netmask"`
	Scope   string `json:"scope" yaml:"scope"`
}

// InstanceStateNetworkCounters represents packet counters as part of the network section of a LXD
// instance's state.
//
// API extension: instances
type InstanceStateNetworkCounters struct {
	BytesReceived   int64 `json:"bytes_received" yaml:"bytes_received"`
	BytesSent       int64 `json:"bytes_sent" yaml:"bytes_sent"`
	PacketsReceived int64 `json:"packets_received" yaml:"packets_received"`
	PacketsSent     int64 `json:"packets_sent" yaml:"packets_sent"`
}
