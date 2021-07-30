package e2e

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
)

var destroy = flag.Bool("destroy", false, "a bool")
var nodeOs = flag.String("node_os", "centos8", "a string")
var externalDb = flag.String("external_db", "mysql", "a string")
var arch = flag.String("arch", "amd64", "a string")
var clusterType = flag.String("cluster_type", "etcd", "a string")
var resourceName = flag.String("resource_name", "etcd", "a string")
var sshuser = flag.String("sshuser", "ubuntu", "a string")
var sshkey = flag.String("sshkey", "", "a string")

var (
	kubeconfig string
	masterIPs  string
	workerIPs  string
)

func DeployWorkloads(arch, Kubeconfig string) {
	resource_dir := ""
	if arch == "amd64" {
		resource_dir = "./amd64_resource_files"
	} else {
		resource_dir = "./arm_resource_files"
	}

	files, err := ioutil.ReadDir(resource_dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		workload := filepath.Join(resource_dir, f.Name())
		_, _ = DeployWorkload(workload, Kubeconfig)
	}
}

// nodeOs: ubuntu centos7 centos8 sles15
// clusterType arm, etcd externaldb, if external_db var is not "" picks database from the vars file,
// resourceName: name to resource created timestamp attached

func BuildCluster(nodeOs, clusterType, externalDb, resourceName string, t *testing.T, destroy bool) (string, string, string) {

	tDir := "./terraform/modules/k3scluster"
	vDir := "/config/" + nodeOs + clusterType + ".tfvars"

	if externalDb != "" {
		vDir = "/config/" + nodeOs + externalDb + ".tfvars"
	}

	tfDir, _ := filepath.Abs(tDir)
	varDir, _ := filepath.Abs(vDir)
	TerraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
		Vars: map[string]interface{}{
			"cluster_type":  clusterType,
			"resource_name": resourceName,
			"external_db":   externalDb,
		},
	}

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(t, TerraformOptions)
		return "", "", ""
	}

	fmt.Printf("Creating Cluster")
	terraform.InitAndApply(t, TerraformOptions)
	kubeconfig := terraform.Output(t, TerraformOptions, "kubeconfig") + "_kubeconfig"
	masterIps := terraform.Output(t, TerraformOptions, "master_ips")
	workerIps := terraform.Output(t, TerraformOptions, "worker_ips")
	kubeconfigFile := "/config/" + kubeconfig
	return kubeconfigFile, masterIps, workerIps
}
