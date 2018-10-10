module k8s.io/api

require (
	github.com/gogo/protobuf v0.0.0-20170330071051-c0656edd0d9e
	github.com/stretchr/testify v0.0.0-20180319223459-c679ae2cc0cb
)

require k8s.io/apimachinery v0.0.0-20180913025736-6dd46049f395

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
