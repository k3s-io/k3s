region = "us-east-2"
qa_space = ""
create_lb = false

external_db_version = "5.7"
instance_class = "db.t2.micro"
db_group_name = "mysql5.7"
engine_mode = "provisioned"
db_username = ""
db_password = ""

username = ""
password = ""

ec2_instance_class = "t3a.medium"
vpc_id = ""
subnets = ""
availability_zone = "us-east-2a"
sg_id = ""

no_of_server_nodes = 2
no_of_worker_nodes = 1
server_flags = "token: test"
worker_flags = "token: test"

k3s_version = "v1.23.8+k3s2"
install_mode = "INSTALL_K3S_VERSION"
environment = "local"