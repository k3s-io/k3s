module github.com/rancher/k3s

go 1.16

replace (
	github.com/Microsoft/hcsshim => github.com/Microsoft/hcsshim v0.8.20
	github.com/benmoss/go-powershell => github.com/k3s-io/go-powershell v0.0.0-20201118222746-51f4c451fbd7
	github.com/containerd/aufs => github.com/containerd/aufs v1.0.0
	github.com/containerd/btrfs => github.com/containerd/btrfs v1.0.0
	github.com/containerd/cgroups => github.com/containerd/cgroups v1.0.1
	github.com/containerd/console => github.com/containerd/console v1.0.2
	github.com/containerd/containerd => github.com/k3s-io/containerd v1.5.7-k3s1 // k3s-release/1.5
	github.com/containerd/continuity => github.com/k3s-io/continuity v0.0.0-20210309170710-f93269e0d5c1
	github.com/containerd/fifo => github.com/containerd/fifo v1.0.0
	github.com/containerd/go-runc => github.com/containerd/go-runc v1.0.0
	github.com/containerd/ttrpc => github.com/containerd/ttrpc v1.0.2
	github.com/containerd/typeurl => github.com/containerd/typeurl v1.0.2
	github.com/containerd/zfs => github.com/containerd/zfs v1.0.0
	github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/docker/distribution => github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker => github.com/docker/docker v20.10.2+incompatible
	github.com/docker/libnetwork => github.com/docker/libnetwork v0.8.0-dev.2.0.20190624125649-f0e46a78ea34
	github.com/golang/protobuf => github.com/golang/protobuf v1.5.2
	github.com/juju/errors => github.com/k3s-io/nocode v0.0.0-20200630202308-cb097102c09f
	github.com/kubernetes-sigs/cri-tools => github.com/k3s-io/cri-tools v1.21.0-k3s1
	github.com/matryer/moq => github.com/rancher/moq v0.0.0-20190404221404-ee5226d43009
	// LOOK TO scripts/download FOR THE VERSION OF runc THAT WE ARE BUILDING/SHIPPING
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.0.2
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v1.0.3-0.20210316141917-a8c4a9ee0f6b
	github.com/rancher/k3s/pkg/data => ./pkg/data
	github.com/rancher/wrangler => github.com/rancher/wrangler v0.8.5
	go.etcd.io/etcd/api/v3 => github.com/k3s-io/etcd/api/v3 v3.5.0-k3s2
	go.etcd.io/etcd/client/v3 => github.com/k3s-io/etcd/client/v3 v3.5.0-k3s2
	go.etcd.io/etcd/etcdutl/v3 => github.com/k3s-io/etcd/etcdutl/v3 v3.5.0-k3s2
	go.etcd.io/etcd/server/v3 => github.com/k3s-io/etcd/server/v3 v3.5.0-k3s2
	golang.org/x/crypto => golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/net => golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4
	golang.org/x/sys => golang.org/x/sys v0.0.0-20210616094352-59db8d763f22
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20200224152610-e50cd9704f63
	google.golang.org/grpc => google.golang.org/grpc v1.38.0
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.2.2
	k8s.io/api => github.com/k3s-io/kubernetes/staging/src/k8s.io/api v1.22.2-k3s1
	k8s.io/apiextensions-apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v1.22.2-k3s1
	k8s.io/apimachinery => github.com/k3s-io/kubernetes/staging/src/k8s.io/apimachinery v1.22.2-k3s1
	k8s.io/apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/apiserver v1.22.2-k3s1
	k8s.io/cli-runtime => github.com/k3s-io/kubernetes/staging/src/k8s.io/cli-runtime v1.22.2-k3s1
	k8s.io/client-go => github.com/k3s-io/kubernetes/staging/src/k8s.io/client-go v1.22.2-k3s1
	k8s.io/cloud-provider => github.com/k3s-io/kubernetes/staging/src/k8s.io/cloud-provider v1.22.2-k3s1
	k8s.io/cluster-bootstrap => github.com/k3s-io/kubernetes/staging/src/k8s.io/cluster-bootstrap v1.22.2-k3s1
	k8s.io/code-generator => github.com/k3s-io/kubernetes/staging/src/k8s.io/code-generator v1.22.2-k3s1
	k8s.io/component-base => github.com/k3s-io/kubernetes/staging/src/k8s.io/component-base v1.22.2-k3s1
	k8s.io/component-helpers => github.com/k3s-io/kubernetes/staging/src/k8s.io/component-helpers v1.22.2-k3s1
	k8s.io/controller-manager => github.com/k3s-io/kubernetes/staging/src/k8s.io/controller-manager v1.22.2-k3s1
	k8s.io/cri-api => github.com/k3s-io/kubernetes/staging/src/k8s.io/cri-api v1.22.2-k3s1
	k8s.io/csi-translation-lib => github.com/k3s-io/kubernetes/staging/src/k8s.io/csi-translation-lib v1.22.2-k3s1
	k8s.io/klog => github.com/k3s-io/klog v1.0.0-k3s1 // k3s-release-1.x
	k8s.io/klog/v2 => github.com/k3s-io/klog/v2 v2.9.0-k3s1 // k3s-main
	k8s.io/kube-aggregator => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-aggregator v1.22.2-k3s1
	k8s.io/kube-controller-manager => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-controller-manager v1.22.2-k3s1
	k8s.io/kube-proxy => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-proxy v1.22.2-k3s1
	k8s.io/kube-scheduler => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-scheduler v1.22.2-k3s1
	k8s.io/kubectl => github.com/k3s-io/kubernetes/staging/src/k8s.io/kubectl v1.22.2-k3s1
	k8s.io/kubelet => github.com/k3s-io/kubernetes/staging/src/k8s.io/kubelet v1.22.2-k3s1
	k8s.io/kubernetes => github.com/k3s-io/kubernetes v1.22.2-k3s1
	k8s.io/legacy-cloud-providers => github.com/k3s-io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v1.22.2-k3s1
	k8s.io/metrics => github.com/k3s-io/kubernetes/staging/src/k8s.io/metrics v1.22.2-k3s1
	k8s.io/mount-utils => github.com/k3s-io/kubernetes/staging/src/k8s.io/mount-utils v1.22.2-k3s1
	k8s.io/node-api => github.com/k3s-io/kubernetes/staging/src/k8s.io/node-api v1.22.2-k3s1
	k8s.io/pod-security-admission => github.com/k3s-io/kubernetes/staging/src/k8s.io/pod-security-admission v1.22.2-k3s1
	k8s.io/sample-apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-apiserver v1.22.2-k3s1
	k8s.io/sample-cli-plugin => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-cli-plugin v1.22.2-k3s1
	k8s.io/sample-controller => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-controller v1.22.2-k3s1
	mvdan.cc/unparam => mvdan.cc/unparam v0.0.0-20210104141923-aac4ce9116a7
)

require (
	github.com/Microsoft/hcsshim v0.8.21
	github.com/containerd/cgroups v1.0.1
	github.com/containerd/containerd v1.5.7
	github.com/containerd/fuse-overlayfs-snapshotter v1.0.3
	github.com/containerd/stargz-snapshotter v0.8.0
	github.com/coreos/go-iptables v0.5.0
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f
	github.com/docker/docker v20.10.7+incompatible
	github.com/erikdubbelboer/gspt v0.0.0-20190125194910-e68493906b83
	github.com/flannel-io/flannel v0.14.1-0.20210827074410-fca1560c91cc
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golangplus/testing v1.0.0 // indirect
	github.com/google/cadvisor v0.39.2
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/k3s-io/helm-controller v0.11.5
	github.com/k3s-io/kine v0.8.0
	github.com/klauspost/compress v1.13.5
	github.com/kubernetes-sigs/cri-tools v0.0.0-00010101000000-000000000000
	github.com/lib/pq v1.10.2
	github.com/mattn/go-sqlite3 v1.14.8
	github.com/minio/minio-go/v7 v7.0.7
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.11.0
	// LOOK TO scripts/download FOR THE VERSION OF runc THAT WE ARE BUILDING/SHIPPING
	github.com/opencontainers/runc v1.0.2
	github.com/opencontainers/selinux v1.8.2
	github.com/pierrec/lz4 v2.6.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/rancher/dynamiclistener v0.2.6
	github.com/rancher/lasso v0.0.0-20210616224652-fc3ebd901c08
	github.com/rancher/remotedialer v0.2.0
	github.com/rancher/wharfie v0.3.4
	github.com/rancher/wrangler v0.8.3
	github.com/robfig/cron/v3 v3.0.1
	github.com/rootless-containers/rootlesskit v0.14.5
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/tchap/go-patricia v2.3.0+incompatible // indirect
	github.com/urfave/cli v1.22.2
	go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	go.etcd.io/etcd/etcdutl/v3 v3.5.0
	go.etcd.io/etcd/server/v3 v3.5.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
	google.golang.org/grpc v1.40.0
	gopkg.in/yaml.v2 v2.4.0
	inet.af/tcpproxy v0.0.0-20200125044825-b6bb9b5b8252
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/apiserver v0.22.2
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cloud-provider v0.22.2
	k8s.io/component-base v0.22.2
	k8s.io/controller-manager v0.22.2 // indirect
	k8s.io/cri-api v0.22.2
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kubectl v0.22.2
	k8s.io/kubernetes v1.22.2
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/yaml v1.2.0
)
