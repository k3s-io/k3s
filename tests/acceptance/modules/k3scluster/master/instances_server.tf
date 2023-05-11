resource "aws_db_instance" "db" {
  count                  = (var.cluster_type == "etcd" || var.external_db == "" || var.external_db == "NULL" ? 0 : (var.external_db != "" && var.external_db != "aurora-mysql" ? 1 : 0))
  identifier             = "${var.resource_name}${local.random_string}-db"
  storage_type           = "gp2"
  allocated_storage      = 20
  engine                 = var.external_db
  engine_version         = var.external_db_version
  instance_class         = var.instance_class
  db_name                = "mydb"
  parameter_group_name   = var.db_group_name
  username               = var.db_username
  password               = var.db_password
  availability_zone      = var.availability_zone
  tags = {
    Environment = var.environment
  }
  skip_final_snapshot    = true
}

resource "aws_rds_cluster" "db" {
  count                  = (var.external_db == "aurora-mysql" && var.cluster_type == "" ? 1 : 0)
  cluster_identifier     = "${var.resource_name}${local.random_string}-db"
  engine                 = var.external_db
  engine_version         = var.external_db_version
  availability_zones     = [var.availability_zone]
  database_name          = "mydb"
  master_username        = var.db_username
  master_password        = var.db_password
  engine_mode            = var.engine_mode
  tags = {
    Environment          = var.environment
  }
  skip_final_snapshot    = true
}

resource "aws_rds_cluster_instance" "db" {
 count                   = (var.external_db == "aurora-mysql" && var.cluster_type == "" ? 1 : 0)
 cluster_identifier      = aws_rds_cluster.db[0].id
 identifier              = "${var.resource_name}${local.random_string}-instance1"
 instance_class          = var.instance_class
  engine                 = aws_rds_cluster.db[0].engine
  engine_version         = aws_rds_cluster.db[0].engine_version
}

resource "aws_instance" "master" {
  ami                    = var.aws_ami
  instance_type          = var.ec2_instance_class
  connection {
    type                 = "ssh"
    user                 = var.aws_user
    host                 = self.public_ip
    private_key          = file(var.access_key)
  }
  root_block_device {
    volume_size          = "20"
    volume_type          = "standard"
  }
  subnet_id              = var.subnets
  availability_zone      = var.availability_zone
  vpc_security_group_ids = [var.sg_id]
  key_name               = var.key_name
  tags = {
    Name                 = "${var.resource_name}-server"
  }
  provisioner "file" {
    source = "install/install_k3s_master.sh"
    destination = "/tmp/install_k3s_master.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/install_k3s_master.sh",
      "sudo /tmp/install_k3s_master.sh ${var.node_os} ${var.create_lb ? aws_route53_record.aws_route53[0].fqdn : self.public_ip} ${var.install_mode} ${var.k3s_version} ${var.cluster_type == "" ? var.external_db : "etcd"} ${self.public_ip} \"${data.template_file.test.rendered}\" \"${var.server_flags}\"  ${var.username} ${var.password}",
    ]
  }
  provisioner "local-exec" {
    command = "echo ${aws_instance.master.public_ip} >/tmp/${var.resource_name}_master_ip"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/nodetoken /tmp/${var.resource_name}_nodetoken"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/config /tmp/${var.resource_name}_config"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/joinflags /tmp/${var.resource_name}_joinflags"
  }
  provisioner "local-exec" {
    command = "scp -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ${var.access_key} ${var.aws_user}@${aws_instance.master.public_ip}:/tmp/master_cmd /tmp/${var.resource_name}_master_cmd"
  }
  provisioner "local-exec" {
    command = "sed s/127.0.0.1/\"${var.create_lb ? aws_route53_record.aws_route53[0].fqdn : aws_instance.master.public_ip}\"/g /tmp/${var.resource_name}_config >/tmp/${var.resource_name}_kubeconfig"
  }
}

data "template_file" "test" {
  template   = (var.cluster_type == "etcd" ? "NULL": (var.external_db == "postgres" ? "postgres://${aws_db_instance.db[0].username}:${aws_db_instance.db[0].password}@${aws_db_instance.db[0].endpoint}/${aws_db_instance.db[0].db_name}" : (var.external_db == "aurora-mysql" ? "mysql://${aws_rds_cluster.db[0].master_username}:${aws_rds_cluster.db[0].master_password}@tcp(${aws_rds_cluster.db[0].endpoint})/${aws_rds_cluster.db[0].database_name}" : "mysql://${aws_db_instance.db[0].username}:${aws_db_instance.db[0].password}@tcp(${aws_db_instance.db[0].endpoint})/${aws_db_instance.db[0].db_name}")))
  depends_on = [data.template_file.test_status]
}

data "template_file" "test_status" {
  template = (var.cluster_type == "etcd" ? "NULL": ((var.external_db == "postgres" ? aws_db_instance.db[0].endpoint : (var.external_db == "aurora-mysql" ? aws_rds_cluster_instance.db[0].endpoint : aws_db_instance.db[0].endpoint))))
}
data "local_file" "token" {
  filename   = "/tmp/${var.resource_name}_nodetoken"
  depends_on = [aws_instance.master]
}

locals {
  node_token = trimspace(data.local_file.token.content)
}

resource "random_string" "suffix" {
  length = 3
  upper = false
  special = false
}

locals {
  random_string =  random_string.suffix.result
}

resource "aws_instance" "master2-ha" {
  ami                    = var.aws_ami
  instance_type          = var.ec2_instance_class
  count                  = var.no_of_server_nodes - 1
  connection {
    type                 = "ssh"
    user                 = var.aws_user
    host                 = self.public_ip
    private_key          = file(var.access_key)
  }
  root_block_device {
    volume_size          = "20"
    volume_type          = "standard"
  }
  subnet_id              = var.subnets
  availability_zone      = var.availability_zone
  vpc_security_group_ids = [var.sg_id]
  key_name               = var.key_name
  depends_on             = [aws_instance.master]
  tags = {
    Name                 = "${var.resource_name}-server-ha${count.index + 1}"
  }
  provisioner "file" {
    source               = "install/join_k3s_master.sh"
    destination          = "/tmp/join_k3s_master.sh"
  }
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/join_k3s_master.sh",
      "sudo /tmp/join_k3s_master.sh ${var.node_os} ${var.create_lb ? aws_route53_record.aws_route53[0].fqdn : "${aws_instance.master.public_ip}"} ${var.install_mode} ${var.k3s_version} ${var.cluster_type} ${self.public_ip} ${aws_instance.master.public_ip} ${local.node_token} \"${data.template_file.test.rendered}\" \"${var.server_flags}\" ${var.username} ${var.password}",
    ]
  }
}

resource "aws_lb_target_group" "aws_tg_80" {
  count              = var.create_lb ? 1 : 0
  port               = 80
  protocol           = "TCP"
  vpc_id             = var.vpc_id
  name               = "${var.resource_name}${local.random_string}-tg-80"
  health_check {
        protocol            = "HTTP"
        port                = "traffic-port"
        path                = "/ping"
        interval            = 10
        timeout             = 6
        healthy_threshold   = 3
        unhealthy_threshold = 3
        matcher             = "200-399"
  }
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_80" {
  count              = var.create_lb ? 1 : 0
  target_group_arn   = aws_lb_target_group.aws_tg_80[0].arn
  target_id          = aws_instance.master.id
  port               = 80
  depends_on         = ["aws_instance.master"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_80_2" {
  target_group_arn   = aws_lb_target_group.aws_tg_80[0].arn
  count              = var.create_lb ? length(aws_instance.master2-ha) : 0
  target_id          = aws_instance.master2-ha[count.index].id
  port               = 80
  depends_on         = ["aws_instance.master"]
}

resource "aws_lb_target_group" "aws_tg_443" {
  count              = var.create_lb ? 1 : 0
  port               = 443
  protocol           = "TCP"
  vpc_id             = var.vpc_id
  name               = "${var.resource_name}${local.random_string}-tg-443"
  health_check {
        protocol            = "HTTP"
        port                = 80
        path                = "/ping"
        interval            = 10
        timeout             = 6
        healthy_threshold   = 3
        unhealthy_threshold = 3
        matcher             = "200-399"
  }
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_443" {
  count              = var.create_lb ? 1 : 0
  target_group_arn   = aws_lb_target_group.aws_tg_443[0].arn
  target_id          = aws_instance.master.id
  port               = 443
  depends_on         = ["aws_instance.master"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_443_2" {
  target_group_arn   = aws_lb_target_group.aws_tg_443[0].arn
  count              = var.create_lb ? length(aws_instance.master2-ha) : 0
  target_id          = aws_instance.master2-ha[count.index].id
  port               = 443
  depends_on         = ["aws_instance.master"]
}

resource "aws_lb_target_group" "aws_tg_6443" {
  count              = var.create_lb ? 1 : 0
  port               = 6443
  protocol           = "TCP"
  vpc_id             = var.vpc_id
  name               = "${var.resource_name}${local.random_string}-tg-6443"
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_6443" {
  count              = var.create_lb ? 1 : 0
  target_group_arn   = aws_lb_target_group.aws_tg_6443[0].arn
  target_id          = aws_instance.master.id
  port               = 6443
  depends_on         = ["aws_instance.master"]
}

resource "aws_lb_target_group_attachment" "aws_tg_attachment_6443_2" {
  target_group_arn   = aws_lb_target_group.aws_tg_6443[0].arn
  count              = var.create_lb ? length(aws_instance.master2-ha) : 0
  target_id          = aws_instance.master2-ha[count.index].id
  port               = 6443
  depends_on         = ["aws_instance.master"]
}

resource "aws_lb" "aws_nlb" {
  count              = var.create_lb ? 1 : 0
  internal           = false
  load_balancer_type = "network"
  subnets            = [var.subnets]
  name               = "${var.resource_name}${local.random_string}-nlb"
}

resource "aws_lb_listener" "aws_nlb_listener_80" {
  count              = var.create_lb ? 1 : 0
  load_balancer_arn  = aws_lb.aws_nlb[0].arn
  port               = "80"
  protocol           = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.aws_tg_80[0].arn
  }
}

resource "aws_lb_listener" "aws_nlb_listener_443" {
  count              = var.create_lb ? 1 : 0
  load_balancer_arn  = aws_lb.aws_nlb[0].arn
  port               = "443"
  protocol           = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.aws_tg_443[0].arn
  }
}

resource "aws_lb_listener" "aws_nlb_listener_6443" {
  count              = var.create_lb ? 1 : 0
  load_balancer_arn  = aws_lb.aws_nlb[0].arn
  port               = "6443"
  protocol           = "TCP"
  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.aws_tg_6443[0].arn
  }
}

resource "aws_route53_record" "aws_route53" {
  count              = var.create_lb ? 1 : 0
  zone_id            = data.aws_route53_zone.selected.zone_id
  name               = "${var.resource_name}${local.random_string}-r53"
  type               = "CNAME"
  ttl                = "300"
  records            = [aws_lb.aws_nlb[0].dns_name]
  depends_on         = ["aws_lb_listener.aws_nlb_listener_6443"]
}

data "aws_route53_zone" "selected" {
  name               = var.qa_space
  private_zone       = false
}