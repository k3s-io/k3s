module k8s.io/code-generator

require (
	github.com/PuerkitoBio/purell v1.0.0 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20160726150825-5bd2802263f2 // indirect
	github.com/davecgh/go-spew v0.0.0-20170626231645-782f4967f2dc // indirect
	github.com/emicklei/go-restful v0.0.0-20170410110728-ff4f55a20633 // indirect
	github.com/go-openapi/jsonpointer v0.0.0-20160704185906-46af16f9f7b1 // indirect
	github.com/go-openapi/jsonreference v0.0.0-20160704190145-13c6e3589ad9 // indirect
	github.com/go-openapi/spec v0.0.0-20180213232550-1de3e0542de6 // indirect
	github.com/go-openapi/swag v0.0.0-20170606142751-f3f9494671f9 // indirect
	github.com/gogo/protobuf v0.0.0-20170330071051-c0656edd0d9e
	github.com/golang/glog v0.0.0-20141105023935-44145f04b68c
	github.com/kr/pretty v0.1.0 // indirect
	github.com/mailru/easyjson v0.0.0-20170624190925-2f5df55504eb // indirect
	github.com/pmezard/go-difflib v0.0.0-20151028094244-d8ed2627bdf0 // indirect
	github.com/spf13/pflag v1.0.1
	github.com/stretchr/testify v0.0.0-20180319223459-c679ae2cc0cb // indirect
	golang.org/x/net v0.0.0-20180906233101-161cd47e91fd // indirect
	golang.org/x/text v0.3.0 // indirect
	golang.org/x/tools v0.0.0-20170428054726-2382e3994d48 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v2 v2.2.1 // indirect
	k8s.io/gengo v0.0.0-20180702041517-fdcf9f9480fd
	k8s.io/kube-openapi v0.0.0-20180711000925-0cf8f7e6ed1d
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
