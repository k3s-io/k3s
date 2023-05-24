package factory

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/k3s-io/k3s/tests/acceptance/shared/util"
	. "github.com/onsi/ginkgo/v2"
)

func BuildCluster(g GinkgoTInterface, destroy bool) (string, error) {
	basepath := util.GetBasepath()
	tfDir, err := filepath.Abs(basepath + util.ModulesPath)
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(basepath + util.TfVarsPath)
	if err != nil {
		return "", err
	}

	terraformOptions := terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}

	util.NumServers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
		"no_of_server_nodes"))
	if err != nil {
		return "", err
	}

	util.NumAgents, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
		"no_of_worker_nodes"))
	if err != nil {
		return "", err
	}

	util.ClusterType = terraform.GetVariableAsStringFromVarFile(g, varDir, "cluster_type")
	util.ExternalDb = terraform.GetVariableAsStringFromVarFile(g, varDir, "external_db")
	util.AwsUser = terraform.GetVariableAsStringFromVarFile(g, varDir, "aws_user")
	util.AccessKey = terraform.GetVariableAsStringFromVarFile(g, varDir, "access_key")

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(g, &terraformOptions)
		return "cluster destroyed", err
	}

	fmt.Printf("Creating Cluster")

	terraform.InitAndApply(g, &terraformOptions)
	util.KubeConfigFile = "/tmp/" + terraform.Output(g, &terraformOptions, "kubeconfig") + "_kubeconfig"
	util.ServerIPs = terraform.Output(g, &terraformOptions, "master_ips")
	util.AgentIPs = terraform.Output(g, &terraformOptions, "worker_ips")
	util.RenderedTemplate = terraform.Output(g, &terraformOptions, "rendered_template")

	return "cluster created", err
}
