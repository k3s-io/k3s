package util

import "flag"

// global configurations
var (
	Arch             = flag.String("arch", "amd64", "a string")
	Destroy          = flag.Bool("destroy", false, "a bool")
	KubeConfigFile   string
	ServerIPs        string
	AgentIPs         string
	NumServers       int
	NumAgents        int
	AwsUser          string
	AccessKey        string
	RenderedTemplate string
	ExternalDb       string
	ClusterType      string

	GetAll            = "kubectl get all -A --kubeconfig="
	GetPodsWide       = "kubectl get pods -o wide --no-headers -A --kubeconfig="
	GetNodesWide      = "kubectl get nodes --no-headers -o wide -A --kubeconfig="
	GetExternalNodeIp = "kubectl get node --output=jsonpath='{range .items[*]} {.status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig="
	Running           = "Running"
)
