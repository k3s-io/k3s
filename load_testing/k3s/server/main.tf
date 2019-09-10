terraform {
  backend "local" {
    path = "server.tfstate"
  }
}

locals {
  name                   = "k3s-load-server"
  node_count             = 1
  k3s_cluster_secret     = "pvc-6476dcaf-73a0-11e9-b8e5-06943b744282"
  install_k3s_version    = "v0.9.0-rc2"
  prom_worker_node_count = 0
  worker_node_count      = 0
}

provider "aws" {
  region  = "us-west-2"
  profile = "rancher-eng"
}

resource "aws_eip" "k3s-server" {
  count = local.node_count
  vpc   = true
}

resource "aws_security_group" "k3s" {
  name   = "${local.name}-rancher-server"
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

module "k3s-server-asg" {
  source               = "terraform-aws-modules/autoscaling/aws"
  version              = "3.0.0"
  name                 = "load-testing-k3s-server"
  asg_name             = "load-testing-k3s-server"
  instance_type        = var.server_instance_type
  image_id             = data.aws_ami.ubuntu.id
  user_data            = data.template_file.k3s-server-user_data.rendered
  ebs_optimized        = true
  iam_instance_profile = aws_iam_instance_profile.k3s-server.name

  desired_capacity    = local.node_count
  health_check_type   = "EC2"
  max_size            = local.node_count
  min_size            = local.node_count
  vpc_zone_identifier = [data.aws_subnet.selected.id]
  spot_price          = "1.591"

  security_groups = [
    aws_security_group.k3s.id,
  ]

  lc_name = "load-testing-k3s-server"

  root_block_device = [
    {
      volume_size = "1000"
      volume_type = "gp2"
    },
  ]
}

module "k3s-prom-worker-asg" {
  source               = "terraform-aws-modules/autoscaling/aws"
  version              = "3.0.0"
  name                 = "load-testing-k3s-prom-worker"
  asg_name             = "load-testing-k3s-prom-worker"
  instance_type        = "m5.large"
  image_id             = data.aws_ami.ubuntu.id
  user_data            = data.template_file.k3s-prom-worker-user_data.rendered
  ebs_optimized        = true
  iam_instance_profile = aws_iam_instance_profile.k3s-server.name

  desired_capacity    = local.prom_worker_node_count
  health_check_type   = "EC2"
  max_size            = local.prom_worker_node_count
  min_size            = local.prom_worker_node_count
  vpc_zone_identifier = [data.aws_subnet.selected.id]
  spot_price          = "0.340"

  security_groups = [
    aws_security_group.k3s.id,
  ]

  lc_name = "load-testing-k3s-prom-worker"

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
    command     = "until ssh ubuntu@${aws_eip.k3s-server.0.public_ip} 'sudo sed \"s/localhost/${aws_eip.k3s-server.0.public_ip}/g;s/127.0.0.1/${aws_eip.k3s-server.0.public_ip}/g\" /etc/rancher/k3s/k3s.yaml' >| ../cluster-loader/kubeConfig.yaml; do sleep 5; done"
  }
}
