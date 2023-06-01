package factory

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gruntwork-io/terratest/modules/terraform"
	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

var (
	once    sync.Once
	cluster *Cluster
)

type Cluster struct {
	Status           string
	ServerIPs        []string
	AgentIPs         []string
	NumServers       int
	NumAgents        int
	RenderedTemplate string
	ExternalDb       string
	ClusterType      string
}

// NewCluster creates a new cluster and returns his values from terraform config and vars
func NewCluster(g GinkgoTInterface) (*Cluster, error) {
	basepath := shared.GetBasepath()
	tfDir, err := filepath.Abs(basepath + "/acceptance/modules/k3scluster")
	if err != nil {
		return nil, err
	}
	varDir, err := filepath.Abs(basepath + "/acceptance/modules/k3scluster/config/local.tfvars")
	if err != nil {
		return nil, err
	}

	terraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}

	NumServers, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir, "no_of_server_nodes"))
	if err != nil {
		return nil, err
	}

	NumAgents, err := strconv.Atoi(terraform.GetVariableAsStringFromVarFile(g, varDir, "no_of_worker_nodes"))
	if err != nil {
		return nil, err
	}

	fmt.Println("Creating Cluster")

	terraform.InitAndApply(g, terraformOptions)

	ClusterType := terraform.GetVariableAsStringFromVarFile(g, varDir, "cluster_type")
	ExternalDb := terraform.GetVariableAsStringFromVarFile(g, varDir, "external_db")
	ServerIPs := strings.Split(terraform.Output(g, terraformOptions, "master_ips"), ",")
	AgentIPs := strings.Split(terraform.Output(g, terraformOptions, "worker_ips"), ",")
	RenderedTemplate := terraform.Output(g, terraformOptions, "rendered_template")
	shared.AwsUser = terraform.GetVariableAsStringFromVarFile(g, varDir, "aws_user")
	shared.AccessKey = terraform.GetVariableAsStringFromVarFile(g, varDir, "access_key")
	shared.KubeConfigFile = "/tmp/" + terraform.Output(g, terraformOptions, "kubeconfig") + "_kubeconfig"

	return &Cluster{
		Status:           "cluster created",
		ServerIPs:        ServerIPs,
		AgentIPs:         AgentIPs,
		NumServers:       NumServers,
		NumAgents:        NumAgents,
		RenderedTemplate: RenderedTemplate,
		ExternalDb:       ExternalDb,
		ClusterType:      ClusterType,
	}, nil
}

// GetCluster returns a singleton cluster
func GetCluster(g GinkgoTInterface) *Cluster {
	var err error
	once.Do(func() {
		cluster, err = NewCluster(g)
		if err != nil {
			g.Errorf("error getting cluster: %v", err)
		}
	})
	return cluster
}

// DestroyCluster destroys the cluster and returns a message
func DestroyCluster(g GinkgoTInterface) (string, error) {
	basepath := shared.GetBasepath()
	tfDir, err := filepath.Abs(basepath + "/modules/k3scluster")
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(basepath + "/modules/k3scluster/config/local.tfvars")
	if err != nil {
		return "", err
	}

	terraformOptions := terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
	}
	terraform.Destroy(g, &terraformOptions)

	return "cluster destroyed", nil
}
