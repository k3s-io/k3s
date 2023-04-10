username = "ShylajaD"
password = "shyAdmin"

region                = "us-east-2"
qa_space              = "qa.rancher.space"
create_lb             = false


#node_os            = "sles15"
#aws_ami             = "ami-046cd3113e0c1b581"

#node_os            = "oracle8"
#aws_ami            = "ami-054a49e0c0c7fce5c"

#ami-0283a57753b18025b
node_os             = "ubuntu"
aws_ami             = "ami-0283a57753b18025b"

#aws_user            = "ec2-user"
aws_user            = "ubuntu"
#aws_user            = "cloud-user"



##############  external db variables  #################

#external_db = "postgres"
#external_db_version   = "14.6"
#db_group_name = "default.postgres14"
#instance_class = "db.t3.micro"

#aurora-mysql
external_db           = "aurora-mysql"
external_db_version   = "5.7.mysql_aurora.2.11.2"
instance_class        = "db.t3.medium"
db_group_name         = "default.aurora-mysql5.7"

# mysql
#external_db           = "mysql"
#external_db_version   = "8.0.32"
#instance_class        = "db.t3.micro"
#db_group_name         = "default.mysql8.0"

## mariadb
#external_db           = "mariadb"
#external_db_version   = "10.6.11"
#instance_class        = "db.t3.medium"
#db_group_name         = "default.mariadb10.6"

engine_mode           = "provisioned"
db_username           = "adminuser"
db_password           = "admin1234"


# AWS variables
ec2_instance_class    = "t3.medium"
vpc_id                = "vpc-bfccf4d7"
subnets               = "subnet-ee8cac86"
availability_zone     = "us-east-2a"
sg_id                 = "sg-0e753fd5550206e55"
#iam_role = "RancherK8SUnrestrictedCloudProviderRoleUS"
#\nprotect-kernel-defaults: true\nselinux: true
server_flags          = "token: test"
worker_flags          = "token: test"

# cluster_type "" = using external db | cluster_type "etcd" = using etcd
cluster_type          = "etcd"
#version or commit value
k3s_version           = "v1.25.2+k3s1"
#valid options: 'latest', 'stable' (default), 'testing'
k3s_channel           = "stable"
#valid options: 'INSTALL_K3S_VERSION', 'INSTALL_K3S_COMMIT'var.external_db
install_mode          = "INSTALL_K3S_VERSION"
environment           = "local"

key_name              = "jenkins-elliptic-validation"

#Run locally use this access_key bellow
access_key            = "/Users/moral/jenkins-keys/jenkins-elliptic-validation.pem"

#Run with docker use this access_key bellow
#access_key            = "/go/src/github.com/k3s-io/k3s/tests/acceptance/modules/k3scluster/config/.ssh/aws_key.pem"

#Variable to load path on docker
#access_key_local      = "jenkinskeypath"


##################  Please be careful with the following variables and configuration  ##################
## split_roles must be always true if you want to split the roles
## nodes must be always filled with the total number of nodes or 0
# role_order is the order in which the roles will be assigned to the nodes
# Numbers 1-6 correspond to: all-roles (1), etcd-only (2), etcd-cp (3), etcd-worker (4), cp-only (5), cp-worker (6)
split_roles        = false
no_of_server_nodes = 1
no_of_worker_nodes = 1
etcd_only_nodes    = 0
etcd_cp_nodes      = 0
etcd_worker_nodes  = 0
cp_only_nodes      = 0
cp_worker_nodes    = 0
role_order         = "1,2,3,4,5"



#Add unique resource name to avoid conflicts
resource_name         = "franlocalk3s"