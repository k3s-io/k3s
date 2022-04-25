output "Registration_address" {
  value = "${data.local_file.master_ip.content}"
}

output "master_node_token" {
  value = "${data.local_file.token.content}"
}

output "worker_ips" {
  value = join("," , aws_instance.worker.*.public_ip)
  description = "The public IP of the AWS node"
}
