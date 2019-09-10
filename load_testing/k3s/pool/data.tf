data "terraform_remote_state" "server" {
  backend = "local"

  config = {
    path = "${path.module}/../server/server.tfstate"
  }
}

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

data "template_file" "k3s-pool-worker-user_data" {
  template = file("${path.module}/files/pool_worker_userdata.tmpl")

  vars = {
    k3s_url             = data.terraform_remote_state.server.outputs.public_ip[0]
    k3s_cluster_secret  = local.k3s_cluster_secret
    install_k3s_version = local.install_k3s_version
    k3s_per_node        = var.k3s_per_node
  }
}
