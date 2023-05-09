variable "aws_ami" {}
variable "aws_user" {}
variable "region" {}
variable "access_key" {}
variable "vpc_id" {}
variable "subnets" {}
variable "availability_zone" {}
variable "sg_id" {}
variable "qa_space" {}
variable "ec2_instance_class" {}
variable "resource_name" {}
variable "key_name" {}
variable "external_db" {}
variable "external_db_version" {}
variable "instance_class" {}
variable "db_group_name" {}
variable "username" {}
variable "password" {}
variable "k3s_version" {}
variable "no_of_server_nodes" {}
variable "server_flags" {}

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
}
variable "role_order" {
  description = "Comma separated order of how to bring the nodes up when split roles"
  type = string
}
variable "all_role_nodes" {}
variable "etcd_only_nodes" {}
variable "etcd_cp_nodes" {}
variable "etcd_worker_nodes" {}
variable "cp_only_nodes" {}
variable "cp_worker_nodes" {}