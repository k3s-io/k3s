variable "node_count" {
  description = "Number of nodes to run k3s agents on."
  type        = number
  # default   = 10
}

variable "k3s_per_node" {
  description = "Number of k3s agent docker containers to run per ec2 instance"
  type        = number
  default     = 10
}

variable "worker_instance_type" {
  type    = string
  default = "c5.4xlarge"
}

variable "extra_ssh_keys" {
  type        = list
  default     = []
  description = "Extra ssh keys to inject into Rancher instances"
}
