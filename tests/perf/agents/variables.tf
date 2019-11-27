variable "agent_node_count" {
  description = "Number of nodes to run k3s agents on."
  type        = number
  # default   = 10
}

variable "agent_instance_type" {
  type    = string
  default = "t3.2xlarge"
}

variable "extra_ssh_keys" {
  type        = list
  default     = []
  description = "Extra ssh keys to inject into Rancher instances"
}

variable "k3s_version" {
  default     = "v0.9.1"
  type        = string
  description = "Version of K3S to install"
}

variable "name" {
  default     = "k3s-loadtest"
  type        = string
  description = "Name to identify this cluster"
}

variable "k3s_cluster_secret" {
  type        = string
  description = "Cluster secret for k3s cluster registration"
}