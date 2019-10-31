variable "server_instance_type" {
  # default = "c4.8xlarge"
}

variable "k3s_version" {
  default     = "v0.9.1"
  type        = string
  description = "Version of K3S to install"
}

variable "k3s_server_args" {
  default = ""
}

variable "prom_worker_node_count" {
  default     = 0
  type        = number
  description = "The number of workers to create labeled for prometheus"
}

variable "k3s_cluster_secret" {
  default     = "pvc-6476dcaf-73a0-11e9-b8e5-06943b744282"
  type        = string
  description = "Cluster secret for k3s cluster registration"
}
variable "prom_host" {
  default = ""
}
variable "graf_host" {
  default = ""
}
variable "name" {
  default     = "k3s-loadtest"
  type        = string
  description = "Name to identify this cluster"
}

variable "ssh_key_path" {
  default = "~/.ssh/id_rsa"
  type = string
  description = "Path of the private key to ssh to the nodes"
}

variable "extra_ssh_keys" {
  type        = list
  default     = []
  description = "Extra ssh keys to inject into Rancher instances"
}

variable "k3s_ha" {
  default     = 0
  description = "Enable k3s in HA mode"
}

variable "db_instance_type" {
}

variable "db_name" {
  default = "k3s"
}

variable "db_username" {
  default = "postgres"
}

variable "db_password" {
  default = "b58bf234c4bd0133fc7a92b782e498a6"
}

variable "master_count" {
  default     = 1
  description = "Count of k3s master servers"
}

variable "debug" {
  default     = 0
  description = "Enable Debug log"
}
