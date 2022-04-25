terraform {
  backend "local" {
    path = "pool.tfstate"
  }
}

locals {
  name                = var.name
  k3s_cluster_secret  = var.k3s_cluster_secret
}

provider "aws" {
  region  = "us-east-2"
  profile = "rancher-eng"
}

resource "aws_security_group" "k3s" {
  name   = "${local.name}-pool"
  vpc_id = data.aws_vpc.default.id

  ingress {
    from_port   = 22
    to_port     = 22
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

module "k3s-pool-worker-asg" {
  source        = "terraform-aws-modules/autoscaling/aws"
  version       = "3.0.0"
  name          = "${local.name}-pool"
  asg_name      = "${local.name}-pool"
  instance_type = var.agent_instance_type
  image_id      = data.aws_ami.ubuntu.id
  user_data     = base64encode(templatefile("${path.module}/files/pool_worker_userdata.tmpl", { k3s_url = data.terraform_remote_state.server.outputs.public_ip, k3s_cluster_secret = local.k3s_cluster_secret, extra_ssh_keys = var.extra_ssh_keys, install_k3s_version = var.k3s_version }))
  ebs_optimized = true

  default_cooldown          = 10
  health_check_grace_period = 30
  wait_for_capacity_timeout = "60m"

  desired_capacity    = var.agent_node_count
  health_check_type   = "EC2"
  max_size            = var.agent_node_count
  min_size            = var.agent_node_count
  vpc_zone_identifier = [data.aws_subnet.selected.id]
  spot_price          = "0.680"

  security_groups = [
    aws_security_group.k3s.id,
  ]

  lc_name = "${local.name}-pool"

  root_block_device = [
    {
      volume_size = "30"
      volume_type = "gp2"
    },
  ]
}
