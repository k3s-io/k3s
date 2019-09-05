module github.com/rancher/k3s

go 1.12

replace (
	github.com/containerd/containerd => github.com/rancher/containerd v1.2.8-k3s.1
	github.com/containerd/cri => github.com/rancher/cri v1.2.8-k3s.1
	github.com/containernetworking/plugins => github.com/rancher/plugins v0.7.5-k3s1
	github.com/coreos/flannel => github.com/ibuildthecloud/flannel v0.10.1-0.20190131215433-823afe66b226
	github.com/kubernetes-sigs/cri-tools => github.com/rancher/cri-tools v1.15.0-k3s.2
	github.com/matryer/moq => github.com/rancher/moq v0.0.0-20190404221404-ee5226d43009
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v0.0.0-20180911193056-5684b8af48c1
	github.com/rancher/dynamiclistener => github.com/erikwilson/rancher-dynamiclistener v0.0.0-20190717164634-c08b499d1719
	github.com/rancher/kine => github.com/ibuildthecloud/kine v0.1.0
	golang.org/x/sys => github.com/golang/sys v0.0.0-20190204203706-41f3e6584952
	k8s.io/api => github.com/rancher/kubernetes/staging/src/k8s.io/api v1.15.3-k3s.3
	k8s.io/apiextensions-apiserver => github.com/rancher/kubernetes/staging/src/k8s.io/apiextensions-apiserver v1.15.3-k3s.3
	k8s.io/apimachinery => github.com/rancher/kubernetes/staging/src/k8s.io/apimachinery v1.15.3-k3s.3
	k8s.io/apiserver => github.com/rancher/kubernetes/staging/src/k8s.io/apiserver v1.15.3-k3s.3
	k8s.io/cli-runtime => github.com/rancher/kubernetes/staging/src/k8s.io/cli-runtime v1.15.3-k3s.3
	k8s.io/client-go => github.com/rancher/kubernetes/staging/src/k8s.io/client-go v1.15.3-k3s.3
	k8s.io/cloud-provider => github.com/rancher/kubernetes/staging/src/k8s.io/cloud-provider v1.15.3-k3s.3
	k8s.io/cluster-bootstrap => github.com/rancher/kubernetes/staging/src/k8s.io/cluster-bootstrap v1.15.3-k3s.3
	k8s.io/code-generator => github.com/rancher/kubernetes/staging/src/k8s.io/code-generator v1.15.3-k3s.3
	k8s.io/component-base => github.com/rancher/kubernetes/staging/src/k8s.io/component-base v1.15.3-k3s.3
	k8s.io/cri-api => github.com/rancher/kubernetes/staging/src/k8s.io/cri-api v1.15.3-k3s.3
	k8s.io/csi-translation-lib => github.com/rancher/kubernetes/staging/src/k8s.io/csi-translation-lib v1.15.3-k3s.3
	k8s.io/kube-aggregator => github.com/rancher/kubernetes/staging/src/k8s.io/kube-aggregator v1.15.3-k3s.3
	k8s.io/kube-controller-manager => github.com/rancher/kubernetes/staging/src/k8s.io/kube-controller-manager v1.15.3-k3s.3
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190228160746-b3a7cee44a30
	k8s.io/kube-proxy => github.com/rancher/kubernetes/staging/src/k8s.io/kube-proxy v1.15.3-k3s.3
	k8s.io/kube-scheduler => github.com/rancher/kubernetes/staging/src/k8s.io/kube-scheduler v1.15.3-k3s.3
	k8s.io/kubectl => github.com/rancher/kubernetes/staging/src/k8s.io/kubectl v1.15.3-k3s.3
	k8s.io/kubelet => github.com/rancher/kubernetes/staging/src/k8s.io/kubelet v1.15.3-k3s.3
	k8s.io/kubernetes => github.com/rancher/kubernetes v1.15.3-k3s.3
	k8s.io/legacy-cloud-providers => github.com/rancher/kubernetes/staging/src/k8s.io/legacy-cloud-providers v1.15.3-k3s.3
	k8s.io/metrics => github.com/rancher/kubernetes/staging/src/k8s.io/metrics v1.15.3-k3s.3
	k8s.io/node-api => github.com/rancher/kubernetes/staging/src/k8s.io/node-api v1.15.3-k3s.3
	k8s.io/sample-apiserver => github.com/rancher/kubernetes/staging/src/k8s.io/sample-apiserver v1.15.3-k3s.3
	k8s.io/sample-cli-plugin => github.com/rancher/kubernetes/staging/src/k8s.io/sample-cli-plugin v1.15.3-k3s.3
	k8s.io/sample-controller => github.com/rancher/kubernetes/staging/src/k8s.io/sample-controller v1.15.3-k3s.3
	sigs.k8s.io/structured-merge-diff => sigs.k8s.io/structured-merge-diff v0.0.0-20190302045857-e85c7b244fd2
)

require (
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/alexflint/go-filemutex v0.0.0-20171022225611-72bdc8eae2ae // indirect
	github.com/bhendo/go-powershell v0.0.0-20190719160123-219e7fb4e41e // indirect
	github.com/buger/jsonparser v0.0.0-20181115193947-bf1c66bbce23 // indirect
	github.com/containerd/cgroups v0.0.0-20190328223300-4994991857f9 // indirect
	github.com/containerd/console v0.0.0-20180822173158-c12b1e7919c1 // indirect
	github.com/containerd/containerd v1.2.8
	github.com/containerd/continuity v0.0.0-20181001140422-bd77b46c8352 // indirect
	github.com/containerd/cri v1.11.1
	github.com/containerd/fifo v0.0.0-20180307165137-3d5202aec260 // indirect
	github.com/containerd/go-cni v0.0.0-20181011142537-40bcf8ec8acd // indirect
	github.com/containerd/go-runc v0.0.0-20180907222934-5a6d9f37cfa3 // indirect
	github.com/containerd/ttrpc v0.0.0-20190513141551-f82148331ad2 // indirect
	github.com/containernetworking/plugins v0.8.2
	github.com/coreos/flannel v0.11.0
	github.com/coreos/go-iptables v0.4.2 // indirect
	github.com/coreos/go-systemd v0.0.0-20180511133405-39ca1b05acc7
	github.com/docker/distribution v0.0.0-20190205005809-0d3efadf0154 // indirect
	github.com/docker/docker v0.7.3-0.20190327010347-be7ac8be2ae0
	github.com/docker/go-events v0.0.0-20170721190031-9461782956ad // indirect
	github.com/docker/go-metrics v0.0.0-20180131145841-4ea375f7759c // indirect
	github.com/emicklei/go-restful v2.2.1+incompatible // indirect
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-sql-driver/mysql v1.4.1
	github.com/gofrs/flock v0.7.1 // indirect
	github.com/gogo/googleapis v1.0.0 // indirect
	github.com/google/cadvisor v0.34.0 // indirect
	github.com/google/tcpproxy v0.0.0-20180808230851-dfa16c61dad2
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/websocket v1.4.0
	github.com/hashicorp/errwrap v0.0.0-20141028054710-7554cd9344ce // indirect
	github.com/hashicorp/go-multierror v0.0.0-20161216184304-ed905158d874 // indirect
	github.com/j-keck/arping v1.0.0 // indirect
	github.com/juju/errors v0.0.0-20190806202954-0232dcc7464d // indirect
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8 // indirect
	github.com/juju/testing v0.0.0-20190723135506-ce30eb24acd2 // indirect
	github.com/kubernetes-sigs/cri-tools v1.15.0
	github.com/lib/pq v1.1.1
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/mindprince/gonvml v0.0.0-20190828220739-9ebdce4bb989 // indirect
	github.com/mistifyio/go-zfs v0.0.0-20171122051224-166add352731 // indirect
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/opencontainers/go-digest v0.0.0-20180430190053-c9281466c8b2 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.0.0-20181113202123-f000fe11ece1
	github.com/opencontainers/runtime-tools v0.6.0 // indirect
	github.com/opencontainers/selinux v1.2.2 // indirect
	github.com/pkg/errors v0.8.1
	github.com/rakelkar/gonetsh v0.0.0-20190719023240-501daadcadf8 // indirect
	github.com/rancher/dynamiclistener v0.0.0-20190717164634-c08b499d1719
	github.com/rancher/helm-controller v0.2.2
	github.com/rancher/kine v0.0.0-00010101000000-000000000000
	github.com/rancher/remotedialer v0.2.0
	github.com/rancher/wrangler v0.2.0
	github.com/rancher/wrangler-api v0.2.0
	github.com/rootless-containers/rootlesskit v0.6.0
	github.com/seccomp/libseccomp-golang v0.0.0-20160531183505-32f571b70023 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.1
	github.com/syndtr/gocapability v0.0.0-20170704070218-db04d3cc01c8 // indirect
	github.com/tchap/go-patricia v2.2.6+incompatible // indirect
	github.com/theckman/go-flock v0.7.1 // indirect
	github.com/urfave/cli v1.21.0
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20180618132009-1d523034197f // indirect
	go.etcd.io/bbolt v1.3.1-etcd.8 // indirect
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/sys v0.0.0-20190616124812-15dcb6c0061f
	google.golang.org/grpc v1.20.1
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/apiserver v0.0.0
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/component-base v0.0.0
	k8s.io/cri-api v0.0.0
	k8s.io/klog v0.3.1
	k8s.io/kubernetes v1.15.3
	k8s.io/utils v0.0.0-20190829053155-3a4a5477acf8 // indirect
)
