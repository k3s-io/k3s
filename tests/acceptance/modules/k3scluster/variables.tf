#variable "db" {}
variable "no_of_worker_nodes" {}
variable "aws_ami" {}
variable "aws_user" {}
variable "region" {}
variable "access_key" {}
variable "vpc_id" {}
variable "subnets" {}
variable "qa_space" {}
variable "resource_name" {}
variable "key_name" {}
variable "external_db" {}
variable "external_db_version" {}
variable "instance_class" {}
variable "ec2_instance_class" {}
variable "db_group_name" {}
variable "username" {}
variable "password" {}
variable "k3s_version" {}
variable "no_of_server_nodes" {}
variable "server_flags" {}
variable "worker_flags" {}
variable "availability_zone" {}
variable "sg_id" {}
variable "cluster_type" {}
variable "node_os" {}
variable "db_username" {}
variable "db_password" {}
variable "environment" {}
variable "engine_mode" {}
variable "install_mode" {}
variable "create_lb" {
  description = "Create Network Load Balancer if set to true"
  type = bool
}
variable "split_roles" {
  description = "When true, server nodes may be a mix of etcd, cp, and worker"
  type = bool
  default = false
}
variable "role_order" {
  description = "Comma separated order of how to bring the nodes up when split roles"
  type = string
  default = "1,2,3,4,5,6"
}
variable "etcd_only_nodes" {
  default = 0
}
variable "etcd_cp_nodes" {
  default = 0
}
variable "etcd_worker_nodes" {
  default = 0
}
variable "cp_only_nodes" {
  default = 0
}
variable "cp_worker_nodes" {
  default = 0
}