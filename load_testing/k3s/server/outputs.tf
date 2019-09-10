output "public_ip" {
  value = aws_eip.k3s-server.*.public_ip
}

output "install_k3s_version" {
  value = local.install_k3s_version
}

output "k3s_cluster_secret" {
  value = local.k3s_cluster_secret
}
