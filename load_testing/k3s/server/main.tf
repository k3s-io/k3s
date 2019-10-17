terraform {
  backend "local" {
    path = "server.tfstate"
  }
}

locals {
  name                   = var.name
  k3s_cluster_secret     = var.k3s_cluster_secret
  install_k3s_version    = var.k3s_version
  prom_worker_node_count = var.prom_worker_node_count
}

provider "aws" {
  region  = "us-west-2"
  profile = "rancher-eng"
}

resource "aws_security_group" "k3s" {
  name   = "${local.name}-sg"
  vpc_id = data.aws_vpc.default.id

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "TCP"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 6443
    to_port     = 6443
    protocol    = "TCP"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port = 0
    to_port   = 0
    protocol  = "-1"
    self      = true
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_spot_instance_request" "k3s-server" {
  instance_type = var.server_instance_type
  ami           = data.aws_ami.ubuntu.id
  user_data     = base64encode(templatefile("${path.module}/files/server_userdata.tmpl", { extra_ssh_keys = var.extra_ssh_keys, public_ip = aws_spot_instance_request.k3s-server.public_ip, metrics_yaml = base64encode(data.template_file.metrics.rendered), prom_yaml = base64encode(data.template_file.k3s-prom-yaml.rendered), k3s_cluster_secret = local.k3s_cluster_secret, install_k3s_version = local.install_k3s_version, k3s_server_args = var.k3s_server_args }))

  ebs_optimized        = true
  wait_for_fulfillment = true
  security_groups = [
    aws_security_group.k3s.id,
  ]

  root_block_device {
    volume_size = "1000"
    volume_type = "gp2"
  }

  tags = {
    Name = "${local.name}-server"
  }
}

module "k3s-prom-worker-asg" {
  source        = "terraform-aws-modules/autoscaling/aws"
  version       = "3.0.0"
  name          = "${local.name}-prom-worker"
  asg_name      = "${local.name}-prom-worker"
  instance_type = "m5.large"
  image_id      = data.aws_ami.ubuntu.id
  user_data     = base64encode(templatefile("${path.module}/files/worker_userdata.tmpl", { extra_ssh_keys = var.extra_ssh_keys, k3s_url = aws_spot_instance_request.k3s-server.public_ip, k3s_cluster_secret = local.k3s_cluster_secret, install_k3s_version = local.install_k3s_version, k3s_exec = "--node-label prom=true" }))
  ebs_optimized = true

  desired_capacity    = local.prom_worker_node_count
  health_check_type   = "EC2"
  max_size            = local.prom_worker_node_count
  min_size            = local.prom_worker_node_count
  vpc_zone_identifier = [data.aws_subnet.selected.id]
  spot_price          = "0.340"

  security_groups = [
    aws_security_group.k3s.id,
  ]

  lc_name = "${local.name}-prom-worker"

  root_block_device = [
    {
      volume_size = "100"
      volume_type = "gp2"
    },
  ]
}

resource "null_resource" "get-kubeconfig" {
  provisioner "local-exec" {
    interpreter = ["bash", "-c"]
    command     = "until ssh ubuntu@${aws_spot_instance_request.k3s-server.public_ip} 'sudo sed \"s/localhost/$aws_spot_instance_request.k3s-server.public_ip}/g;s/127.0.0.1/${aws_spot_instance_request.k3s-server.public_ip}/g\" /etc/rancher/k3s/k3s.yaml' >| ../cluster-loader/kubeConfig.yaml; do sleep 5; done"
  }
}
