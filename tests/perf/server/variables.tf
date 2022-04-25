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
  type        = string
  description = "Cluster secret for k3s cluster registration"
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

variable "server_ha" {
  default     = 0
  description = "Enable k3s in HA mode"
}

variable "etcd_count" {
  default = 3
}

variable "db_engine" {
  default = "postgres"
}

variable "db_instance_type" {
}

variable "db_name" {
  default = "k3s"
}

variable "db_username" {
  default = "postgres"
}

variable "db_password" {}

variable "db_version" {}

variable "server_count" {
  default     = 1
  description = "Count of k3s master servers"
}

variable "debug" {
  default     = 0
  description = "Enable Debug log"
}

variable "prom_worker_instance_type" {
  default = "m5.large"
  description = "Prometheus instance type"
}

variable "domain_name" {
  description = "FQDN of the cluster"
}

variable "zone_id" {
  description = "route53 zone id to register the domain name"
}
