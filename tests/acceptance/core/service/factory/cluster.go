package factory

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/gruntwork-io/terratest/modules/terraform"
	util2 "github.com/k3s-io/k3s/tests/acceptance/shared/util"
	"github.com/onsi/ginkgo/v2"
)

func BuildCluster(g ginkgo.GinkgoTInterface, destroy bool) (string, error) {
	basepath := util2.GetBasepath()
	tfDir, err := filepath.Abs(basepath + util2.ModulesPath)
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(basepath + util2.TfVarsPath)
	if err != nil {
		return "", err
	}

	terraformOptions := terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}

	util2.NumServers, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
		"no_of_server_nodes"))
	if err != nil {
		return "", err
	}

	util2.NumAgents, err = strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
		"no_of_worker_nodes"))
	if err != nil {
		return "", err
	}

	splitRoles := terraform.GetVariableAsStringFromVarFile(g, varDir, "split_roles")
	if splitRoles == "true" {
		etcdNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_only_nodes"))
		if err != nil {
			return "", err
		}
		etcdCpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_cp_nodes"))
		if err != nil {
			return "", err
		}
		etcdWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"etcd_worker_nodes"))
		if err != nil {
			return "", err
		}
		cpNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"cp_only_nodes"))
		if err != nil {
			return "", err
		}
		cpWorkerNodes, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir,
			"cp_worker_nodes"))
		if err != nil {
			return "", err
		}
		util2.NumServers = util2.NumServers + etcdNodes + etcdCpNodes + etcdWorkerNodes +
			+cpNodes + cpWorkerNodes
	}

	util2.ClusterType = terraform.GetVariableAsStringFromVarFile(g, varDir, "cluster_type")
	util2.ExternalDb = terraform.GetVariableAsStringFromVarFile(g, varDir, "external_db")
	util2.AwsUser = terraform.GetVariableAsStringFromVarFile(g, varDir, "aws_user")
	util2.AccessKey = terraform.GetVariableAsStringFromVarFile(g, varDir, "access_key")
	util2.Version = terraform.GetVariableAsStringFromVarFile(g, varDir, "k3s_version")

	if destroy {
		fmt.Printf("Cluster is being deleted")
		terraform.Destroy(g, &terraformOptions)
		return "cluster destroyed", err
	}

	fmt.Printf("Creating Cluster")

	terraform.InitAndApply(g, &terraformOptions)
	util2.KubeConfigFile = "/tmp/" + terraform.Output(g, &terraformOptions, "kubeconfig") + "_kubeconfig"
	util2.ServerIPs = terraform.Output(g, &terraformOptions, "master_ips")
	util2.AgentIPs = terraform.Output(g, &terraformOptions, "worker_ips")
	util2.RenderedTemplate = terraform.Output(g, &terraformOptions, "rendered_template")

	return "cluster created", err
}
