output "public_ip" {
  value = aws_lb.k3s-master-nlb.dns_name
}

output "install_k3s_version" {
  value = local.install_k3s_version
}

output "k3s_cluster_secret" {
  value = local.k3s_cluster_secret
}

output "k3s_server_ips" {
  value = join(",", aws_spot_instance_request.k3s-server.*.public_ip)
}
