package api

// Cluster represents high-level information about a LXD cluster.
//
// API extension: clustering
type Cluster struct {
	ServerName string `json:"server_name" yaml:"server_name"`
	Enabled    bool   `json:"enabled" yaml:"enabled"`

	// API extension: clustering_join
	MemberConfig []ClusterMemberConfigKey `json:"member_config" yaml:"member_config"`
}

// ClusterMemberConfigKey represents a single config key that a new member of
// the cluster is required to provide when joining.
//
// The Value field is empty when getting clustering information with GET
// /1.0/cluster, and should be filled by the joining node when performing a PUT
// /1.0/cluster join request.
//
// API extension: clustering_join
type ClusterMemberConfigKey struct {
	Entity      string `json:"entity" yaml:"entity"`
	Name        string `json:"name" yaml:"name"`
	Key         string `json:"key" yaml:"key"`
	Value       string `json:"value" yaml:"value"`
	Description string `json:"description" yaml:"description"`
}

// ClusterPut represents the fields required to bootstrap or join a LXD
// cluster.
//
// API extension: clustering
type ClusterPut struct {
	Cluster            `yaml:",inline"`
	ClusterAddress     string `json:"cluster_address" yaml:"cluster_address"`
	ClusterCertificate string `json:"cluster_certificate" yaml:"cluster_certificate"`

	// API extension: clustering_join
	ServerAddress   string `json:"server_address" yaml:"server_address"`
	ClusterPassword string `json:"cluster_password" yaml:"cluster_password"`
}

// ClusterMemberPost represents the fields required to rename a LXD node.
//
// API extension: clustering
type ClusterMemberPost struct {
	ServerName string `json:"server_name" yaml:"server_name"`
}

// ClusterMember represents the a LXD node in the cluster.
//
// API extension: clustering
type ClusterMember struct {
	ServerName string `json:"server_name" yaml:"server_name"`
	URL        string `json:"url" yaml:"url"`
	Database   bool   `json:"database" yaml:"database"`
	Status     string `json:"status" yaml:"status"`
	Message    string `json:"message" yaml:"message"`

	// API extension: clustering_roles
	Roles []string `json:"roles" yaml:"roles"`
}
