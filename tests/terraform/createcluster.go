package terraform

import (
	"flag"
	"fmt"

	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

var destroy = flag.Bool("destroy", false, "a bool")
var awsAmi = flag.String("aws_ami", "", "a valid ami string like ami-abcxyz123")
var nodeOs = flag.String("node_os", "ubuntu", "a string")
var externalDb = flag.String("external_db", "mysql", "a string")
var arch = flag.String("arch", "amd64", "a string")
var clusterType = flag.String("cluster_type", "etcd", "a string")
var resourceName = flag.String("resource_name", "etcd", "a string")
var sshuser = flag.String("sshuser", "ubuntu", "a string")
var sshkey = flag.String("sshkey", "", "a string")
var access_key = flag.String("access_key", "", "local path to the private sshkey")
var tfVars = flag.String("tfvars", "./modules/k3scluster/config/local.tfvars", "custom .tfvars file")
var serverNodes = flag.Int("no_of_server_nodes", 2, "count of server nodes")
var workerNodes = flag.Int("no_of_worker_nodes", 1, "count of worker nodes")
var failed = false

var (
	kubeConfigFile string
	masterIPs      string
	workerIPs      string
)

func BuildCluster(nodeOs, awsAmi string, clusterType, externalDb, resourceName string, t *testing.T, destroy bool, arch string) (string, error) {
	tDir := "./modules/k3scluster"

	tfDir, err := filepath.Abs(tDir)
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(*tfVars)
	if err != nil {
		return "", err
	}
	TerraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
		Vars: map[string]interface{}{
			"node_os":            nodeOs,
			"aws_ami":            awsAmi,
			"cluster_type":       clusterType,
			"resource_name":      resourceName,
			"external_db":        externalDb,
			"aws_user":           *sshuser,
			"key_name":           *sshkey,
			"access_key":         *access_key,
			"no_of_server_nodes": *serverNodes,
			"no_of_worker_nodes": *workerNodes,
		},
	}

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(t, TerraformOptions)
		return "cluster destroyed", err
	}

	fmt.Printf("Creating Cluster")
	terraform.InitAndApply(t, TerraformOptions)
	kubeConfigFile = "/tmp/" + terraform.Output(t, TerraformOptions, "kubeconfig") + "_kubeconfig"
	masterIPs = terraform.Output(t, TerraformOptions, "master_ips")
	workerIPs = terraform.Output(t, TerraformOptions, "worker_ips")
	return "cluster created", err
}
