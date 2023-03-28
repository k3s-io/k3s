package createcluster

import (
	"fmt"
	"strconv"

	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	tf "github.com/k3s-io/k3s/tests/terraform"
)

var (
	KubeConfigFile   string
	MasterIPs        string
	WorkerIPs        string
	NumServers       int
	NumWorkers       int
	AwsUser          string
	AccessKey        string
	RenderedTemplate string
	ExternalDb       string
	ClusterType      string
	TfVarsPath       = "/tests/terraform/modules/k3scluster/config/local.tfvars"
	modulesPath      = "/tests/terraform/modules/k3scluster"
)

func BuildCluster(t *testing.T, destroy bool) (string, error) {
	basepath := tf.GetBasepath()
	tfDir, err := filepath.Abs(basepath + modulesPath)
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(basepath + TfVarsPath)
	if err != nil {
		return "", err
	}
	TerraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}

	NumServers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
		"no_of_server_nodes"))
	if err != nil {
		return "", err
	}

	NumWorkers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(t, varDir,
		"no_of_worker_nodes"))
	if err != nil {
		return "", err
	}

	ClusterType = terraform.GetVariableAsStringFromVarFile(t, varDir, "cluster_type")
	ExternalDb = terraform.GetVariableAsStringFromVarFile(t, varDir, "external_db")
	AwsUser = terraform.GetVariableAsStringFromVarFile(t, varDir, "aws_user")
	AccessKey = terraform.GetVariableAsStringFromVarFile(t, varDir, "access_key")

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(t, TerraformOptions)
		return "cluster destroyed", err
	}

	fmt.Printf("Creating Cluster")

	terraform.InitAndApply(t, TerraformOptions)
	KubeConfigFile = "/tmp/" + terraform.Output(t, TerraformOptions, "kubeconfig") + "_kubeconfig"
	MasterIPs = terraform.Output(t, TerraformOptions, "master_ips")
	WorkerIPs = terraform.Output(t, TerraformOptions, "worker_ips")
	RenderedTemplate = terraform.Output(t, TerraformOptions, "rendered_template")

	return "cluster created", err
}
