package versionbump

var (
	ExpectedValueUpgradedHost string
	ExpectedValueUpgradedNode string
	CmdHost                   string
	ExpectedValueHost         string
	CmdNode                   string
	ExpectedValueNode         string
	Description               string
	GetImageLocalPath         = "kubectl describe pod -n kube-system local-path-provisioner- "
	GetPodTestWithAnnotations = "kubectl get pod test-pod -o yaml --kubeconfig="
)

const (
	K3sVersion        = "k3s --version"
	FlannelBinVersion = "/var/lib/rancher/k3s/data/current/bin/flannel"
	CNIbin            = "/var/lib/rancher/k3s/data/current/bin/cni"
	GrepImage         = " | grep -i Image"
	GrepAnnotations   = " | grep -A2 annotations "
)
