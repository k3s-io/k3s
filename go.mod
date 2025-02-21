module github.com/k3s-io/k3s

go 1.23.3

replace (
	github.com/Microsoft/hcsshim => github.com/Microsoft/hcsshim v0.12.9
	github.com/Mirantis/cri-dockerd => github.com/k3s-io/cri-dockerd v0.3.15-k3s1.32-1 // v1.32.0-k3s1
	github.com/cilium/ebpf => github.com/cilium/ebpf v0.12.3
	github.com/cloudnativelabs/kube-router/v2 => github.com/k3s-io/kube-router/v2 v2.2.1
	github.com/containerd/containerd => github.com/k3s-io/containerd v1.7.23-k3s2
	github.com/containerd/containerd/api => github.com/containerd/containerd/api v1.8.0
	github.com/containerd/containerd/v2 => github.com/k3s-io/containerd/v2 v2.0.2-k3s2
	github.com/containerd/imgcrypt => github.com/containerd/imgcrypt v1.1.11
	github.com/distribution/reference => github.com/distribution/reference v0.5.0
	github.com/docker/distribution => github.com/docker/distribution v2.8.3+incompatible
	github.com/docker/docker => github.com/docker/docker v25.0.6+incompatible
	github.com/emicklei/go-restful/v3 => github.com/emicklei/go-restful/v3 v3.11.0
	github.com/golang/protobuf => github.com/golang/protobuf v1.5.4
	github.com/google/cadvisor => github.com/k3s-io/cadvisor v0.51.0-k3s1
	github.com/googleapis/gax-go/v2 => github.com/googleapis/gax-go/v2 v2.12.0
	github.com/open-policy-agent/opa => github.com/open-policy-agent/opa v0.59.0 // github.com/Microsoft/hcsshim using bad version v0.42.2
	github.com/opencontainers/runc => github.com/k3s-io/runc v1.2.4-k3s1
	github.com/opencontainers/selinux => github.com/opencontainers/selinux v1.11.0
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v1.19.1
	github.com/prometheus/common => github.com/prometheus/common v0.55.0
	github.com/spegel-org/spegel => github.com/k3s-io/spegel v0.0.30-k3s1
	github.com/ugorji/go => github.com/ugorji/go v1.2.11
	go.etcd.io/etcd/api/v3 => github.com/k3s-io/etcd/api/v3 v3.5.18-k3s1
	go.etcd.io/etcd/client/pkg/v3 => github.com/k3s-io/etcd/client/pkg/v3 v3.5.18-k3s1
	go.etcd.io/etcd/client/v2 => github.com/k3s-io/etcd/client/v2 v2.305.18-k3s1
	go.etcd.io/etcd/client/v3 => github.com/k3s-io/etcd/client/v3 v3.5.18-k3s1
	go.etcd.io/etcd/etcdutl/v3 => github.com/k3s-io/etcd/etcdutl/v3 v3.5.18-k3s1
	go.etcd.io/etcd/pkg/v3 => github.com/k3s-io/etcd/pkg/v3 v3.5.18-k3s1
	go.etcd.io/etcd/raft/v3 => github.com/k3s-io/etcd/raft/v3 v3.5.18-k3s1
	go.etcd.io/etcd/server/v3 => github.com/k3s-io/etcd/server/v3 v3.5.18-k3s1
	go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful => go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful v0.44.0
	golang.org/x/crypto => golang.org/x/crypto v0.24.0
	golang.org/x/net => golang.org/x/net v0.26.0
	golang.org/x/sys => golang.org/x/sys v0.21.0
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20230525234035-dd9d682886f9
	google.golang.org/grpc => google.golang.org/grpc v1.65.0
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.6.0
	k8s.io/api => github.com/k3s-io/kubernetes/staging/src/k8s.io/api v1.32.2-k3s1
	k8s.io/apiextensions-apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v1.32.2-k3s1
	k8s.io/apimachinery => github.com/k3s-io/kubernetes/staging/src/k8s.io/apimachinery v1.32.2-k3s1
	k8s.io/apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/apiserver v1.32.2-k3s1
	k8s.io/cli-runtime => github.com/k3s-io/kubernetes/staging/src/k8s.io/cli-runtime v1.32.2-k3s1
	k8s.io/client-go => github.com/k3s-io/kubernetes/staging/src/k8s.io/client-go v1.32.2-k3s1
	k8s.io/cloud-provider => github.com/k3s-io/kubernetes/staging/src/k8s.io/cloud-provider v1.32.2-k3s1
	k8s.io/cluster-bootstrap => github.com/k3s-io/kubernetes/staging/src/k8s.io/cluster-bootstrap v1.32.2-k3s1
	k8s.io/code-generator => github.com/k3s-io/kubernetes/staging/src/k8s.io/code-generator v1.32.2-k3s1
	k8s.io/component-base => github.com/k3s-io/kubernetes/staging/src/k8s.io/component-base v1.32.2-k3s1
	k8s.io/component-helpers => github.com/k3s-io/kubernetes/staging/src/k8s.io/component-helpers v1.32.2-k3s1
	k8s.io/controller-manager => github.com/k3s-io/kubernetes/staging/src/k8s.io/controller-manager v1.32.2-k3s1
	k8s.io/cri-api => github.com/k3s-io/kubernetes/staging/src/k8s.io/cri-api v1.32.2-k3s1
	k8s.io/csi-translation-lib => github.com/k3s-io/kubernetes/staging/src/k8s.io/csi-translation-lib v1.32.2-k3s1
	k8s.io/dynamic-resource-allocation => github.com/k3s-io/kubernetes/staging/src/k8s.io/dynamic-resource-allocation v1.32.2-k3s1
	k8s.io/endpointslice => github.com/k3s-io/kubernetes/staging/src/k8s.io/endpointslice v1.32.2-k3s1
	k8s.io/externaljwt => github.com/k3s-io/kubernetes/staging/src/k8s.io/externaljwt v1.32.2-k3s1
	k8s.io/klog => github.com/k3s-io/klog v1.0.0-k3s2 // k3s-release-1.x
	k8s.io/klog/v2 => github.com/k3s-io/klog/v2 v2.120.1-k3s1 // k3s-main
	k8s.io/kms => github.com/k3s-io/kubernetes/staging/src/k8s.io/kms v1.32.2-k3s1
	k8s.io/kube-aggregator => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-aggregator v1.32.2-k3s1
	k8s.io/kube-controller-manager => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-controller-manager v1.32.2-k3s1
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f
	k8s.io/kube-proxy => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-proxy v1.32.2-k3s1
	k8s.io/kube-scheduler => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-scheduler v1.32.2-k3s1
	k8s.io/kubectl => github.com/k3s-io/kubernetes/staging/src/k8s.io/kubectl v1.32.2-k3s1
	k8s.io/kubelet => github.com/k3s-io/kubernetes/staging/src/k8s.io/kubelet v1.32.2-k3s1
	k8s.io/kubernetes => github.com/k3s-io/kubernetes v1.32.2-k3s1
	k8s.io/metrics => github.com/k3s-io/kubernetes/staging/src/k8s.io/metrics v1.32.2-k3s1
	k8s.io/mount-utils => github.com/k3s-io/kubernetes/staging/src/k8s.io/mount-utils v1.32.2-k3s1
	k8s.io/node-api => github.com/k3s-io/kubernetes/staging/src/k8s.io/node-api v1.32.2-k3s1
	k8s.io/pod-security-admission => github.com/k3s-io/kubernetes/staging/src/k8s.io/pod-security-admission v1.32.2-k3s1
	k8s.io/sample-apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-apiserver v1.32.2-k3s1
	k8s.io/sample-cli-plugin => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-cli-plugin v1.32.2-k3s1
	k8s.io/sample-controller => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-controller v1.32.2-k3s1
	sigs.k8s.io/cri-tools => github.com/k3s-io/cri-tools v1.31.0-k3s2
	sourcegraph.com/sourcegraph/go-diff => github.com/sourcegraph/go-diff v0.6.0
)

require (
	github.com/Microsoft/hcsshim v0.12.9
	github.com/Mirantis/cri-dockerd v0.0.0-00010101000000-000000000000
	github.com/blang/semver/v4 v4.0.0
	github.com/cloudnativelabs/kube-router/v2 v2.0.0-00010101000000-000000000000
	github.com/containerd/cgroups/v3 v3.0.3
	github.com/containerd/containerd/api v1.8.0
	github.com/containerd/containerd/v2 v2.0.2
	github.com/containerd/errdefs v1.0.0
	github.com/containerd/fuse-overlayfs-snapshotter/v2 v2.1.0
	github.com/containerd/stargz-snapshotter v0.16.3
	github.com/containerd/zfs/v2 v2.0.0-rc.0
	github.com/coreos/go-iptables v0.8.0
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/distribution/reference v0.6.0
	github.com/docker/docker v27.1.1+incompatible
	github.com/erikdubbelboer/gspt v0.0.0-20190125194910-e68493906b83
	github.com/flannel-io/flannel v0.25.7
	github.com/fsnotify/fsnotify v1.7.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-logr/logr v1.4.2
	github.com/go-logr/stdr v1.2.3-0.20220714215716-96bad1d688c5
	github.com/go-test/deep v1.0.7
	github.com/google/cadvisor v0.51.0
	github.com/google/go-containerregistry v0.20.2
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/inetaf/tcpproxy v0.0.0-20240214030015-3ce58045626c
	github.com/ipfs/go-ds-leveldb v0.5.0
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/joho/godotenv v1.5.1
	github.com/json-iterator/go v1.1.12
	github.com/k3s-io/api v0.1.0
	github.com/k3s-io/helm-controller v0.16.6
	github.com/k3s-io/kine v0.13.9
	github.com/klauspost/compress v1.17.11
	github.com/libp2p/go-libp2p v0.38.2
	github.com/minio/minio-go/v7 v7.0.83
	github.com/moby/sys/userns v0.1.0
	github.com/mwitkow/go-http-dialer v0.0.0-20161116154839-378f744fb2b8
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/onsi/ginkgo/v2 v2.22.2
	github.com/onsi/gomega v1.36.2
	github.com/opencontainers/runc v1.2.1
	github.com/opencontainers/selinux v1.11.1
	github.com/otiai10/copy v1.7.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.20.5
	github.com/prometheus/common v0.61.0
	github.com/rancher/dynamiclistener v0.6.0-rc1
	github.com/rancher/lasso v0.0.0-20250109193533-00757eec2dbd
	github.com/rancher/permissions v0.0.0-20240523180510-4001d3d637f7
	github.com/rancher/remotedialer v0.4.1
	github.com/rancher/wharfie v0.6.7
	github.com/rancher/wrangler/v3 v3.2.0-rc.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/rootless-containers/rootlesskit v1.0.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spegel-org/spegel v1.0.18
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.10.0
	github.com/urfave/cli v1.22.15
	github.com/urfave/cli/v2 v2.27.5
	github.com/vishvananda/netlink v1.3.1-0.20240905180732-b1ce50cfa9be
	github.com/yl2chen/cidranger v1.0.2
	go.etcd.io/etcd/api/v3 v3.5.18
	go.etcd.io/etcd/client/pkg/v3 v3.5.18
	go.etcd.io/etcd/client/v3 v3.5.18
	go.etcd.io/etcd/etcdutl/v3 v3.5.18
	go.etcd.io/etcd/server/v3 v3.5.18
	go.uber.org/mock v0.5.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.32.0
	golang.org/x/mod v0.22.0
	golang.org/x/net v0.34.0
	golang.org/x/sync v0.10.0
	golang.org/x/sys v0.29.0
	google.golang.org/grpc v1.70.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.32.2
	k8s.io/apimachinery v0.32.2
	k8s.io/apiserver v0.32.2
	k8s.io/cli-runtime v0.32.2
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cloud-provider v0.32.2
	k8s.io/cluster-bootstrap v0.0.0
	k8s.io/component-base v0.32.2
	k8s.io/component-helpers v0.32.2
	k8s.io/cri-api v0.32.2
	k8s.io/cri-client v0.32.2
	k8s.io/klog/v2 v2.130.1
	k8s.io/kube-proxy v0.0.0
	k8s.io/kubectl v0.32.2
	k8s.io/kubelet v0.32.2
	k8s.io/kubernetes v1.32.2
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738
	sigs.k8s.io/cri-tools v0.0.0-00010101000000-000000000000
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/invopop/jsonschema v0.12.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
)

require (
	cel.dev/expr v0.18.0 // indirect
	dario.cat/mergo v1.0.1 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6 // indirect
	github.com/AdamKorcz/go-118-fuzz-build v0.0.0-20231105174938-2b5cbb29f3e2 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/JeffAshton/win_pdh v0.0.0-20161109143554-76bb4ee9f0ab // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/semver/v3 v3.3.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Microsoft/hnslib v0.0.8 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/Rican7/retry v0.3.1 // indirect
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/avast/retry-go/v4 v4.6.0 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/bronze1man/goStrongswanVici v0.0.0-20231128135937-211cef3b0b20 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/checkpoint-restore/checkpointctl v1.3.0 // indirect
	github.com/checkpoint-restore/go-criu/v7 v7.2.0 // indirect
	github.com/cilium/ebpf v0.16.0 // indirect
	github.com/container-storage-interface/spec v1.9.0 // indirect
	github.com/containerd/btrfs/v2 v2.0.0 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/containerd/console v1.0.4 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/go-cni v1.1.12 // indirect
	github.com/containerd/go-runc v1.1.0 // indirect
	github.com/containerd/imgcrypt/v2 v2.0.0-rc.1 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/nri v0.8.0 // indirect
	github.com/containerd/otelttrpc v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.1 // indirect
	github.com/containerd/plugin v1.0.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.16.3 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/containernetworking/cni v1.2.3 // indirect
	github.com/containernetworking/plugins v1.5.1 // indirect
	github.com/containers/ocicrypt v1.2.0 // indirect
	github.com/coreos/go-oidc v2.2.1+incompatible // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.5 // indirect
	github.com/cyphar/filepath-securejoin v0.3.5 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0 // indirect
	github.com/docker/cli v27.3.1+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/elastic/gosigar v0.14.3 // indirect
	github.com/emicklei/go-restful v2.16.0+incompatible // indirect
	github.com/emicklei/go-restful/v3 v3.12.1 // indirect
	github.com/euank/go-kmsg-parser v2.0.0+incompatible // indirect
	github.com/evanphx/json-patch v5.9.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flynn/noise v1.1.0 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.4 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/goccy/go-json v0.10.4 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/cel-go v0.22.0 // indirect
	github.com/google/gnostic-models v0.6.9 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/pprof v0.0.0-20241210010833-40e02aabc2ad // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/hanwen/go-fuse/v2 v2.6.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/arc/v2 v2.0.7 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/huin/goupnp v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/intel/goresctrl v0.8.0 // indirect
	github.com/ipfs/boxo v0.27.0 // indirect
	github.com/ipfs/go-cid v0.4.1 // indirect
	github.com/ipfs/go-datastore v0.6.0 // indirect
	github.com/ipld/go-ipld-prime v0.21.0 // indirect
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.2 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/jbenet/go-temp-err-catcher v0.1.0 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/koron/go-ssdp v0.0.4 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/libopenstorage/openstorage v1.0.0 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0 // indirect
	github.com/libp2p/go-cidranger v1.1.0 // indirect
	github.com/libp2p/go-flow-metrics v0.2.0 // indirect
	github.com/libp2p/go-libp2p-asn-util v0.4.1 // indirect
	github.com/libp2p/go-libp2p-kad-dht v0.28.2 // indirect
	github.com/libp2p/go-libp2p-kbucket v0.6.4 // indirect
	github.com/libp2p/go-libp2p-record v0.2.0 // indirect
	github.com/libp2p/go-libp2p-routing-helpers v0.7.4 // indirect
	github.com/libp2p/go-msgio v0.3.0 // indirect
	github.com/libp2p/go-nat v0.2.0 // indirect
	github.com/libp2p/go-netroute v0.2.2 // indirect
	github.com/libp2p/go-reuseport v0.4.0 // indirect
	github.com/libp2p/go-yamux/v4 v4.0.1 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/marten-seemann/tcp v0.0.0-20210406111302-dfbc87cc63fd // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v1.14.24 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.4.1 // indirect
	github.com/mdlayher/vsock v1.2.1 // indirect
	github.com/miekg/dns v1.1.62 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mikioh/tcpinfo v0.0.0-20190314235526-30a79bb1804b // indirect
	github.com/mikioh/tcpopt v0.0.0-20190314235656-172688c1accc // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mistifyio/go-zfs/v3 v3.0.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/ipvs v1.1.0 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/signal v0.7.1 // indirect
	github.com/moby/sys/symlink v0.3.0 // indirect
	github.com/moby/sys/user v0.3.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multiaddr v0.14.0 // indirect
	github.com/multiformats/go-multiaddr-dns v0.4.1 // indirect
	github.com/multiformats/go-multiaddr-fmt v0.1.0 // indirect
	github.com/multiformats/go-multibase v0.2.0 // indirect
	github.com/multiformats/go-multicodec v0.9.0 // indirect
	github.com/multiformats/go-multihash v0.2.3 // indirect
	github.com/multiformats/go-multistream v0.6.0 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/nats-io/jsm.go v0.0.31-0.20220317133147-fe318f464eee // indirect
	github.com/nats-io/jwt/v2 v2.5.5 // indirect
	github.com/nats-io/nats-server/v2 v2.10.12 // indirect
	github.com/nats-io/nats.go v1.34.0 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/opencontainers/runtime-tools v0.9.1-0.20221107090550-2e043c6bd626 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/petermattis/goid v0.0.0-20240813172612-4fcff4a6cae7 // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/pion/datachannel v1.5.10 // indirect
	github.com/pion/dtls/v2 v2.2.12 // indirect
	github.com/pion/ice/v2 v2.3.37 // indirect
	github.com/pion/interceptor v0.1.37 // indirect
	github.com/pion/logging v0.2.2 // indirect
	github.com/pion/mdns v0.0.12 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.15 // indirect
	github.com/pion/rtp v1.8.10 // indirect
	github.com/pion/sctp v1.8.35 // indirect
	github.com/pion/sdp/v3 v3.0.9 // indirect
	github.com/pion/srtp/v2 v2.0.20 // indirect
	github.com/pion/stun v0.6.1 // indirect
	github.com/pion/transport/v2 v2.2.10 // indirect
	github.com/pion/transport/v3 v3.0.7 // indirect
	github.com/pion/turn/v2 v2.1.6 // indirect
	github.com/pion/webrtc/v3 v3.3.5 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/polydawn/refmt v0.89.0 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/quic-go/qpack v0.5.1 // indirect
	github.com/quic-go/quic-go v0.48.2 // indirect
	github.com/quic-go/webtransport-go v0.8.1-0.20241018022711-4ac2c9250e66 // indirect
	github.com/raulk/go-watchdog v1.3.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.5 // indirect
	github.com/shengdoushi/base58 v1.0.0 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20230803200340-78284954bff6 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/tidwall/btree v1.6.0 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20220101234140-673ab2c3ae75 // indirect
	github.com/vbatts/tar-split v0.11.6 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	github.com/xiang90/probing v0.0.0-20221125231312-a49e3df8f510 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	go.etcd.io/bbolt v1.3.11 // indirect
	go.etcd.io/etcd/client/v2 v2.305.18 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.18 // indirect
	go.etcd.io/etcd/raft/v3 v3.5.18 // indirect
	go.mozilla.org/pkcs7 v0.9.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful v0.42.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.56.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/otel v1.32.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.31.0 // indirect
	go.opentelemetry.io/otel/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/sdk v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/dig v1.18.0 // indirect
	go.uber.org/fx v1.23.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20241217172543-b2144cdd0a67 // indirect
	golang.org/x/oauth2 v0.25.0 // indirect
	golang.org/x/term v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	golang.org/x/tools v0.28.0 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20231211153847-12269c276173 // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20230429144221-925a1e7659e6 // indirect
	gonum.org/v1/gonum v0.15.0 // indirect
	google.golang.org/genproto v0.0.0-20241118233622-e639e219e697 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241209162323-e6fa225c2576 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241223144023-3abc09e42ca8 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.32.2 // indirect
	k8s.io/controller-manager v0.25.4 // indirect
	k8s.io/csi-translation-lib v0.0.0 // indirect
	k8s.io/dynamic-resource-allocation v0.0.0 // indirect
	k8s.io/endpointslice v0.0.0 // indirect
	k8s.io/externaljwt v1.32.0 // indirect
	k8s.io/kms v0.0.0 // indirect
	k8s.io/kube-aggregator v0.32.2 // indirect
	k8s.io/kube-controller-manager v0.0.0 // indirect
	k8s.io/kube-openapi v0.0.0-20241127205056-99599406b04f // indirect
	k8s.io/kube-scheduler v0.0.0 // indirect
	k8s.io/metrics v0.0.0 // indirect
	k8s.io/mount-utils v0.32.2 // indirect
	k8s.io/pod-security-admission v0.0.0 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.31.0 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/knftables v0.0.17 // indirect
	sigs.k8s.io/kustomize/api v0.18.0 // indirect
	sigs.k8s.io/kustomize/kustomize/v5 v5.5.0 // indirect
	sigs.k8s.io/kustomize/kyaml v0.18.1 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.3 // indirect
	tags.cncf.io/container-device-interface v0.8.0 // indirect
	tags.cncf.io/container-device-interface/specs-go v0.8.0 // indirect
)
