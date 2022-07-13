package createcluster

import (
	"fmt"

	"path/filepath"
	"testing"

	"github.com/gruntwork-io/terratest/modules/terraform"
	tf "github.com/k3s-io/k3s/tests/terraform"
)

var (
	KubeConfigFile string
	MasterIPs      string
	WorkerIPs      string
)

type options struct {
	nodeOs       string
	awsAmi       string
	clusterType  string
	resourceName string
	externalDb   string
	sshuser      string
	sshkey       string
	accessKey    string
	serverNodes  int
	workerNodes  int
}

func ClusterOptions(os ...ClusterOption) map[string]interface{} {
	opts := options{}
	for _, o := range os {
		opts = o(opts)
	}
	return map[string]interface{}{
		"node_os":            opts.nodeOs,
		"aws_ami":            opts.awsAmi,
		"cluster_type":       opts.clusterType,
		"resource_name":      opts.resourceName,
		"external_db":        opts.externalDb,
		"aws_user":           opts.sshuser,
		"key_name":           opts.sshkey,
		"access_key":         opts.accessKey,
		"no_of_server_nodes": opts.serverNodes,
		"no_of_worker_nodes": opts.workerNodes,
	}
}

func BuildCluster(t *testing.T, tfVarsPath string, destroy bool, terraformVars map[string]interface{}) (string, error) {
	basepath := tf.GetBasepath()
	tfDir, err := filepath.Abs(basepath + "/tests/terraform/modules/k3scluster")
	if err != nil {
		return "", err
	}
	varDir, err := filepath.Abs(basepath + tfVarsPath)
	if err != nil {
		return "", err
	}
	TerraformOptions := &terraform.Options{
		TerraformDir: tfDir,
		VarFiles:     []string{varDir},
		Vars:         terraformVars,
	}

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
	return "cluster created", err
}

type ClusterOption func(o options) options

func NodeOs(n string) ClusterOption {
	return func(o options) options {
		o.nodeOs = n
		return o
	}
}
func AwsAmi(n string) ClusterOption {
	return func(o options) options {
		o.awsAmi = n
		return o
	}
}
func ClusterType(n string) ClusterOption {
	return func(o options) options {
		o.clusterType = n
		return o
	}
}
func ResourceName(n string) ClusterOption {
	return func(o options) options {
		o.resourceName = n
		return o
	}
}
func ExternalDb(n string) ClusterOption {
	return func(o options) options {
		o.externalDb = n
		return o
	}
}
func Sshuser(n string) ClusterOption {
	return func(o options) options {
		o.sshuser = n
		return o
	}
}
func Sshkey(n string) ClusterOption {
	return func(o options) options {
		o.sshkey = n
		return o
	}
}
func AccessKey(n string) ClusterOption {
	return func(o options) options {
		o.accessKey = n
		return o
	}
}
func ServerNodes(n int) ClusterOption {
	return func(o options) options {
		o.serverNodes = n
		return o
	}
}
func WorkerNodes(n int) ClusterOption {
	return func(o options) options {
		o.workerNodes = n
		return o
	}
}
