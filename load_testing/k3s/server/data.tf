data "aws_vpc" "default" {
  default = true
}

data "aws_subnet_ids" "available" {
  vpc_id = data.aws_vpc.default.id
}

data "aws_subnet" "selected" {
  id = "${tolist(data.aws_subnet_ids.available.ids)[1]}"
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "name"
    values = ["ubuntu-minimal/images/*/ubuntu-bionic-18.04-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }
}

data "template_file" "metrics" {
  template = file("${path.module}/files/metrics.yaml")
}
data "template_file" "k3s-prom-yaml" {
  template = file("${path.module}/files/prom.yaml")
  vars = {
    prom_host = var.prom_host
    graf_host = var.graf_host
  }
}

data "template_file" "k3s-server-user_data" {
  template = file("${path.module}/files/server_userdata.tmpl")

  vars = {
    create_eip          = 1
    metrics_yaml        = base64encode(data.template_file.metrics.rendered)
    prom_yaml           = base64encode(data.template_file.k3s-prom-yaml.rendered)
    eip                 = join(",", aws_eip.k3s-server.*.public_ip)
    k3s_cluster_secret  = local.k3s_cluster_secret
    install_k3s_version = local.install_k3s_version
    k3s_server_args     = var.k3s_server_args
  }
}

data "template_file" "k3s-prom-worker-user_data" {
  template = file("${path.module}/files/worker_userdata.tmpl")

  vars = {
    k3s_url             = aws_eip.k3s-server.0.public_ip
    k3s_cluster_secret  = local.k3s_cluster_secret
    install_k3s_version = local.install_k3s_version
    k3s_exec            = "--node-label prom=true"
  }
}

data "template_file" "k3s-worker-user_data" {
  template = file("${path.module}/files/worker_userdata.tmpl")

  vars = {
    k3s_url             = aws_eip.k3s-server.0.public_ip
    k3s_cluster_secret  = local.k3s_cluster_secret
    install_k3s_version = local.install_k3s_version
    k3s_exec            = ""
  }
}
