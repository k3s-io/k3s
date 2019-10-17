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
