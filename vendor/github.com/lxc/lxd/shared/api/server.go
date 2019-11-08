package api

// ServerEnvironment represents the read-only environment fields of a LXD server
type ServerEnvironment struct {
	Addresses              []string `json:"addresses" yaml:"addresses"`
	Architectures          []string `json:"architectures" yaml:"architectures"`
	Certificate            string   `json:"certificate" yaml:"certificate"`
	CertificateFingerprint string   `json:"certificate_fingerprint" yaml:"certificate_fingerprint"`
	Driver                 string   `json:"driver" yaml:"driver"`
	DriverVersion          string   `json:"driver_version" yaml:"driver_version"`
	Kernel                 string   `json:"kernel" yaml:"kernel"`
	KernelArchitecture     string   `json:"kernel_architecture" yaml:"kernel_architecture"`

	// API extension: kernel_features
	KernelFeatures map[string]string `json:"kernel_features" yaml:"kernel_features"`

	KernelVersion string `json:"kernel_version" yaml:"kernel_version"`

	// API extension: lxc_features
	LXCFeatures map[string]string `json:"lxc_features" yaml:"lxc_features"`

	// API extension: projects
	Project string `json:"project" yaml:"project"`

	Server string `json:"server" yaml:"server"`

	// API extension: clustering
	ServerClustered bool   `json:"server_clustered" yaml:"server_clustered"`
	ServerName      string `json:"server_name" yaml:"server_name"`

	ServerPid      int    `json:"server_pid" yaml:"server_pid"`
	ServerVersion  string `json:"server_version" yaml:"server_version"`
	Storage        string `json:"storage" yaml:"storage"`
	StorageVersion string `json:"storage_version" yaml:"storage_version"`
}

// ServerPut represents the modifiable fields of a LXD server configuration
type ServerPut struct {
	Config map[string]interface{} `json:"config" yaml:"config"`
}

// ServerUntrusted represents a LXD server for an untrusted client
type ServerUntrusted struct {
	APIExtensions []string `json:"api_extensions" yaml:"api_extensions"`
	APIStatus     string   `json:"api_status" yaml:"api_status"`
	APIVersion    string   `json:"api_version" yaml:"api_version"`
	Auth          string   `json:"auth" yaml:"auth"`
	Public        bool     `json:"public" yaml:"public"`

	// API extension: macaroon_authentication
	AuthMethods []string `json:"auth_methods" yaml:"auth_methods"`
}

// Server represents a LXD server
type Server struct {
	ServerPut       `yaml:",inline"`
	ServerUntrusted `yaml:",inline"`

	Environment ServerEnvironment `json:"environment" yaml:"environment"`
}

// Writable converts a full Server struct into a ServerPut struct (filters read-only fields)
func (srv *Server) Writable() ServerPut {
	return srv.ServerPut
}
