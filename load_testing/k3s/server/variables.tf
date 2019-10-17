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

variable "extra_ssh_keys" {
  type        = list
  default     = []
  description = "Extra ssh keys to inject into Rancher instances"
}
