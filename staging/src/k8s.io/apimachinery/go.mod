module k8s.io/apimachinery

require (
	github.com/davecgh/go-spew v0.0.0-20170626231645-782f4967f2dc
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96
	github.com/elazarl/goproxy v0.0.0-20170405201442-c4fc26588b6e
	github.com/evanphx/json-patch v0.0.0-20180908160633-36442dbdb585
	github.com/ghodss/yaml v0.0.0-20150909031657-73d445a93680
	github.com/gogo/protobuf v0.0.0-20170330071051-c0656edd0d9e
	github.com/golang/glog v0.0.0-20141105023935-44145f04b68c
	github.com/golang/groupcache v0.0.0-20160516000752-02826c3e7903
	github.com/golang/protobuf v1.2.0
	github.com/google/gofuzz v0.0.0-20161122191042-44d81051d367
	github.com/googleapis/gnostic v0.0.0-20170729233727-0c5108395e2d
	github.com/hashicorp/golang-lru v0.0.0-20160207214719-a0d98a5f2880
	github.com/json-iterator/go v0.0.0-20180612202835-f2b4162afba3
	github.com/modern-go/reflect2 v0.0.0-20180320133207-05fbef0ca5da
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f
	github.com/pborman/uuid v0.0.0-20150603214016-ca53cad383ca
	github.com/spf13/pflag v1.0.1
	github.com/stretchr/testify v0.0.0-20180319223459-c679ae2cc0cb
	golang.org/x/net v0.0.0-20180906233101-161cd47e91fd
	gopkg.in/inf.v0 v0.9.0
	gopkg.in/yaml.v2 v2.2.1
	k8s.io/kube-openapi v0.0.0-20180711000925-0cf8f7e6ed1d
)

require (
	github.com/kr/pretty v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/onsi/gomega v1.4.2 // indirect
	github.com/pmezard/go-difflib v0.0.0-20151028094244-d8ed2627bdf0 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace (
	k8s.io/api v0.0.0-20181005203742-357ec6384fa7 => ../api
	k8s.io/apiextensions-apiserver v0.0.0-20181005210900-6b7e25efea53 => ../apiextensions-apiserver
	k8s.io/apimachinery v0.0.0-20180913025736-6dd46049f395 => ../apimachinery
	k8s.io/apiserver v0.0.0-20181005205051-9f398e330d7f => ../apiserver
	k8s.io/cli-runtime v0.0.0-20181005211514-7a908ff29916 => ../cli-runtime
	k8s.io/client-go v2.0.0-alpha.0.0.20181005204318-cb4883f3dea0+incompatible => ../client-go
	k8s.io/code-generator v0.0.0-20180823001027-3dcf91f64f63 => ../code-generator
	k8s.io/csi-api v0.0.0-20181005211323-acd5d7181032 => ../csi-api
	k8s.io/kube-aggregator v0.0.0-20181005205513-d21b52dfcd98 => ../kube-aggregator
	k8s.io/kube-controller-manager v0.0.0-20181005212343-5265e578d5ff => ../kube-controller-manager
	k8s.io/kube-proxy v0.0.0-20181005211915-341f0725c667 => ../kube-proxy
	k8s.io/kube-scheduler v0.0.0-20181005212216-d83766c4b997 => ../kube-scheduler
	k8s.io/kubelet v0.0.0-20181005212049-fd18f9c8be0f => ../kubelet
	k8s.io/metrics v0.0.0-20181005211129-e763038d2832 => ../metrics
)
