terraform {
  backend "local" {
    path = "pool.tfstate"
  }
}

locals {
  name                = "load-test-pool"
  k3s_cluster_secret  = "pvc-6476dcaf-73a0-11e9-b8e5-06943b744282"
  install_k3s_version = "v0.9.0-rc2"
}

provider "aws" {
  region  = "us-west-2"
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
  name          = local.name
  asg_name      = local.name
  instance_type = var.worker_instance_type
  image_id      = data.aws_ami.ubuntu.id
  user_data     = data.template_file.k3s-pool-worker-user_data.rendered
  ebs_optimized = true

  desired_capacity    = var.node_count
  health_check_type   = "EC2"
  max_size            = var.node_count
  min_size            = var.node_count
  vpc_zone_identifier = [data.aws_subnet.selected.id]
  spot_price          = "0.680"

  security_groups = [
    aws_security_group.k3s.id,
  ]

  lc_name = local.name

  root_block_device = [
    {
      volume_size = "100"
      volume_type = "gp2"
    },
  ]
}
