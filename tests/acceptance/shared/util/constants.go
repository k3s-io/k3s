package util

import (
	"flag"
)

var (
	Destroy        = flag.Bool("destroy", false, "a bool")
	UpgradeVersion = flag.String("upgradeVersion", "", "Version to upgrade the cluster to")
	Arch           = flag.String("arch", "amd64", "a string")

	ExpectedValueUpgradedHost string
	ExpectedValueUpgradedNode string
	CmdHost                   string
	ExpectedValueHost         string
	CmdNode                   string
	ExpectedValueNode         string
	InstallUpgradeFlag        MultiValueFlag
	Description               string
	TestCase                  TestConfigFlag

	KubeConfigFile   string
	ServerIPs        string
	AgentIPs         string
	NumServers       int
	NumAgents        int
	AwsUser          string
	AccessKey        string
	Version          string
	RenderedTemplate string
	ExternalDb       string
	ClusterType      string
)

// global var CMDs
var (
	GetAll                  = "kubectl get all -A --kubeconfig="
	GetPodsWide             = "kubectl get pods -o wide --no-headers -A --kubeconfig="
	GetNodesWide            = "kubectl get nodes --no-headers -o wide -A --kubeconfig="
	GetAppLoadBalancer      = "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig="
	GetIngress              = "kubectl get ingress -o jsonpath='{.items[0].status.loadBalancer.ingress[*].ip}' --kubeconfig="
	GetIngressRunning       = "kubectl get pods  -l app=ingress -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running  --kubeconfig="
	GetImageLocalPath       = "kubectl describe pod -n kube-system local-path-provisioner- "
	GetExternalNodeIp       = "kubectl get node --output=jsonpath='{range .items[*]} {.status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig="
	GetLoadbalancerSVC      = "kubectl get service nginx-loadbalancer-svc --output jsonpath={.spec.ports[0].port} --kubeconfig="
	GetPodDnsUtils          = "kubectl get pods dnsutils --kubeconfig="
	GetPodVolumeTestRunning = "kubectl get pods -l app=volume-test --field-selector=status.phase=Running --kubeconfig="
	GetNodeport             = "kubectl get pods -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig="
	GetClusterIp            = "kubectl get pods -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig="
	DeletePod               = "kubectl delete pod -l app=volume-test --kubeconfig="
)

// constant names and asserts
const (
	NginxAppClusterIp   = "nginx-app-clusterip"
	TestClusterip       = "test-clusterip"
	NginxClusterIpSVC   = "nginx-clusterip-svc"
	NginxNodePortSVC    = "nginx-nodeport-svc"
	NginxAppNodePortSVC = "nginx-app-nodeport"
	TestNodePort        = "test-nodeport"
	TestDaemonset       = "test-daemonset"
	NginxAppIngress     = "nginx-app-ingress"
	TestIngress         = "test-ingress"
	DnsUtils            = "dnsutils"
	Nslookup            = "kubernetes.default.svc.cluster.local"
	VolumeTest          = "volume-test"
	TestingLocalPath    = "testing local path"

	TestLoadBalancer = "test-loadbalancer"
	RunningAssert    = "Running"
	CompletedAssert  = "Completed"
	InstallHelm      = "curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"
	InstallK3sServer = "curl -sfL https://get.k3s.io | sudo %s  sh -s - server"
	InstallK3sAgent  = "curl -sfL https://get.k3s.io | sudo %s sh -s - agent"
	Upgradek3s       = "curl -sfL https://get.k3s.io | %s=%s sh -s - server --cluster-init --token secret --write-kubeconfig-mode=644"
	K3sVersion       = "k3s --version"
	GrepImage        = " | grep -i Image"
	TfVarsPath       = "/modules/k3scluster/config/local.tfvars"
	ModulesPath      = "/modules/k3scluster"
)
