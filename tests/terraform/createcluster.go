package e2e

import (
	"flag"
	"fmt"

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
var failed = false

var (
	kubeConfigFile string
	masterIPs      string
	workerIPs      string
)

func BuildCluster(nodeOs, clusterType, externalDb, resourceName string, t *testing.T, destroy bool, arch string) (string, string, string, error) {
	tDir := "./modules/k3scluster"
	vDir := "/config/" + nodeOs + clusterType + ".tfvars"

	if externalDb != "" {
		vDir = "/config/" + nodeOs + externalDb + ".tfvars"
	}

	tfDir, err := filepath.Abs(tDir)
	if err != nil {
		return "", "", "", err
	}
	varDir, err := filepath.Abs(vDir)
	if err != nil {
		return "", "", "", err
	}
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
		return "", "", "", err
	}

	fmt.Printf("Creating Cluster")
	terraform.InitAndApply(t, TerraformOptions)
	kubeconfig := terraform.Output(t, TerraformOptions, "kubeconfig") + "_kubeconfig"
	masterIPs := terraform.Output(t, TerraformOptions, "master_ips")
	workerIPs := terraform.Output(t, TerraformOptions, "worker_ips")
	kubeconfigFile := "/config/" + kubeconfig
	return kubeconfigFile, masterIPs, workerIPs, err
}
