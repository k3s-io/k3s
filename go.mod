module github.com/k3s-io/k3s

go 1.20

replace (
	github.com/Microsoft/hcsshim => github.com/Microsoft/hcsshim v0.8.25
	github.com/Mirantis/cri-dockerd => github.com/k3s-io/cri-dockerd v0.3.2-k3s2 // k3s/release-1.27
	github.com/cloudnativelabs/kube-router/v2 => github.com/k3s-io/kube-router/v2 v2.0.1-0.20230508174102-b42e5faded1c
	github.com/containerd/containerd => github.com/k3s-io/containerd v1.7.1-k3s1
	github.com/containerd/stargz-snapshotter => github.com/k3s-io/stargz-snapshotter v0.13.0-k3s1
	github.com/coreos/go-systemd => github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/docker/distribution => github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker => github.com/docker/docker v20.10.24+incompatible
	github.com/docker/libnetwork => github.com/docker/libnetwork v0.8.0-dev.2.0.20190624125649-f0e46a78ea34
	github.com/emicklei/go-restful/v3 => github.com/emicklei/go-restful/v3 v3.9.0
	github.com/golang/protobuf => github.com/golang/protobuf v1.5.3
	github.com/googleapis/gax-go/v2 => github.com/googleapis/gax-go/v2 v2.1.1
	github.com/juju/errors => github.com/k3s-io/nocode v0.0.0-20200630202308-cb097102c09f
	github.com/kubernetes-sigs/cri-tools => github.com/k3s-io/cri-tools v1.26.0-rc.0-k3s1
	github.com/opencontainers/runc => github.com/opencontainers/runc v1.1.7
	github.com/opencontainers/runtime-spec => github.com/opencontainers/runtime-spec v1.0.3-0.20220909204839-494a5a6aca78
	github.com/opencontainers/selinux => github.com/opencontainers/selinux v1.10.1
	github.com/rancher/wrangler => github.com/rancher/wrangler v1.1.1-0.20230425173236-39a4707f0689
	go.etcd.io/etcd/api/v3 => github.com/k3s-io/etcd/api/v3 v3.5.7-k3s1
	go.etcd.io/etcd/client/pkg/v3 => github.com/k3s-io/etcd/client/pkg/v3 v3.5.7-k3s1
	go.etcd.io/etcd/client/v2 => github.com/k3s-io/etcd/client/v2 v2.305.7-k3s1
	go.etcd.io/etcd/client/v3 => github.com/k3s-io/etcd/client/v3 v3.5.7-k3s1
	go.etcd.io/etcd/etcdutl/v3 => github.com/k3s-io/etcd/etcdutl/v3 v3.5.7-k3s1
	go.etcd.io/etcd/pkg/v3 => github.com/k3s-io/etcd/pkg/v3 v3.5.7-k3s1
	go.etcd.io/etcd/raft/v3 => github.com/k3s-io/etcd/raft/v3 v3.5.7-k3s1
	go.etcd.io/etcd/server/v3 => github.com/k3s-io/etcd/server/v3 v3.5.7-k3s1

	go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful => go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful v0.35.0
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc => go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.35.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp => go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.35.1
	go.opentelemetry.io/contrib/propagators/b3 => go.opentelemetry.io/contrib/propagators/b3 v1.13.0
	go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.13.0
	go.opentelemetry.io/otel/exporters/otlp/internal/retry => go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.13.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric => go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.32.1
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc => go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.32.1
	go.opentelemetry.io/otel/exporters/otlp/otlptrace => go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.13.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc => go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.13.0
	go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v0.32.1
	go.opentelemetry.io/otel/sdk => go.opentelemetry.io/otel/sdk v1.13.0
	go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.13.0
	go.opentelemetry.io/proto/otlp => go.opentelemetry.io/proto/otlp v0.19.0
	golang.org/x/crypto => golang.org/x/crypto v0.1.0
	golang.org/x/net => golang.org/x/net v0.8.0
	golang.org/x/sys => golang.org/x/sys v0.6.0
	google.golang.org/genproto => google.golang.org/genproto v0.0.0-20220502173005-c8bf987b8c21
	google.golang.org/grpc => google.golang.org/grpc v1.51.0
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.6.0
	k8s.io/api => github.com/k3s-io/kubernetes/staging/src/k8s.io/api v1.27.2-k3s1
	k8s.io/apiextensions-apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v1.27.2-k3s1
	k8s.io/apimachinery => github.com/k3s-io/kubernetes/staging/src/k8s.io/apimachinery v1.27.2-k3s1
	k8s.io/apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/apiserver v1.27.2-k3s1
	k8s.io/cli-runtime => github.com/k3s-io/kubernetes/staging/src/k8s.io/cli-runtime v1.27.2-k3s1
	k8s.io/client-go => github.com/k3s-io/kubernetes/staging/src/k8s.io/client-go v1.27.2-k3s1
	k8s.io/cloud-provider => github.com/k3s-io/kubernetes/staging/src/k8s.io/cloud-provider v1.27.2-k3s1
	k8s.io/cluster-bootstrap => github.com/k3s-io/kubernetes/staging/src/k8s.io/cluster-bootstrap v1.27.2-k3s1
	k8s.io/code-generator => github.com/k3s-io/kubernetes/staging/src/k8s.io/code-generator v1.27.2-k3s1
	k8s.io/component-base => github.com/k3s-io/kubernetes/staging/src/k8s.io/component-base v1.27.2-k3s1
	k8s.io/component-helpers => github.com/k3s-io/kubernetes/staging/src/k8s.io/component-helpers v1.27.2-k3s1
	k8s.io/controller-manager => github.com/k3s-io/kubernetes/staging/src/k8s.io/controller-manager v1.27.2-k3s1
	k8s.io/cri-api => github.com/k3s-io/kubernetes/staging/src/k8s.io/cri-api v1.27.2-k3s1
	k8s.io/csi-translation-lib => github.com/k3s-io/kubernetes/staging/src/k8s.io/csi-translation-lib v1.27.2-k3s1
	k8s.io/dynamic-resource-allocation => github.com/k3s-io/kubernetes/staging/src/k8s.io/dynamic-resource-allocation v1.27.2-k3s1
	k8s.io/klog => github.com/k3s-io/klog v1.0.0-k3s2 // k3s-release-1.x
	k8s.io/klog/v2 => github.com/k3s-io/klog/v2 v2.90.1-k3s1 // k3s-main
	k8s.io/kms => github.com/k3s-io/kubernetes/staging/src/k8s.io/kms v1.27.2-k3s1
	k8s.io/kube-aggregator => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-aggregator v1.27.2-k3s1
	k8s.io/kube-controller-manager => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-controller-manager v1.27.2-k3s1
	k8s.io/kube-proxy => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-proxy v1.27.2-k3s1
	k8s.io/kube-scheduler => github.com/k3s-io/kubernetes/staging/src/k8s.io/kube-scheduler v1.27.2-k3s1
	k8s.io/kubectl => github.com/k3s-io/kubernetes/staging/src/k8s.io/kubectl v1.27.2-k3s1
	k8s.io/kubelet => github.com/k3s-io/kubernetes/staging/src/k8s.io/kubelet v1.27.2-k3s1
	k8s.io/kubernetes => github.com/k3s-io/kubernetes v1.27.2-k3s1
	k8s.io/legacy-cloud-providers => github.com/k3s-io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v1.27.2-k3s1
	k8s.io/metrics => github.com/k3s-io/kubernetes/staging/src/k8s.io/metrics v1.27.2-k3s1
	k8s.io/mount-utils => github.com/k3s-io/kubernetes/staging/src/k8s.io/mount-utils v1.27.2-k3s1
	k8s.io/node-api => github.com/k3s-io/kubernetes/staging/src/k8s.io/node-api v1.27.2-k3s1
	k8s.io/pod-security-admission => github.com/k3s-io/kubernetes/staging/src/k8s.io/pod-security-admission v1.27.2-k3s1
	k8s.io/sample-apiserver => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-apiserver v1.27.2-k3s1
	k8s.io/sample-cli-plugin => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-cli-plugin v1.27.2-k3s1
	k8s.io/sample-controller => github.com/k3s-io/kubernetes/staging/src/k8s.io/sample-controller v1.27.2-k3s1
	mvdan.cc/unparam => mvdan.cc/unparam v0.0.0-20210104141923-aac4ce9116a7
)

require (
	github.com/Mirantis/cri-dockerd v0.0.0-00010101000000-000000000000
	github.com/cloudnativelabs/kube-router/v2 v2.0.0-00010101000000-000000000000
	github.com/containerd/aufs v1.0.0
	github.com/containerd/cgroups v1.1.0
	github.com/containerd/containerd v1.6.10
	github.com/containerd/fuse-overlayfs-snapshotter v1.0.5
	github.com/containerd/stargz-snapshotter v0.14.3
	github.com/containerd/zfs v1.0.0
	github.com/coreos/go-iptables v0.6.0
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
	github.com/docker/docker v23.0.3+incompatible
	github.com/erikdubbelboer/gspt v0.0.0-20190125194910-e68493906b83
	github.com/flannel-io/flannel v0.22.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-sql-driver/mysql v1.6.0
	github.com/go-test/deep v1.0.7
	github.com/google/cadvisor v0.47.1
	github.com/google/uuid v1.3.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.5.0
	github.com/json-iterator/go v1.1.12
	github.com/k3s-io/helm-controller v0.15.0
	github.com/k3s-io/kine v0.10.1
	github.com/klauspost/compress v1.16.5
	github.com/kubernetes-sigs/cri-tools v0.0.0-00010101000000-000000000000
	github.com/lib/pq v1.10.2
	github.com/mattn/go-sqlite3 v1.14.15
	github.com/minio/minio-go/v7 v7.0.33
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/onsi/ginkgo/v2 v2.9.4
	github.com/onsi/gomega v1.27.6
	github.com/opencontainers/runc v1.1.6
	github.com/opencontainers/selinux v1.11.0
	github.com/otiai10/copy v1.7.0
	github.com/pkg/errors v0.9.1
	github.com/rancher/dynamiclistener v0.3.5
	github.com/rancher/lasso v0.0.0-20221227210133-6ea88ca2fbcc
	github.com/rancher/remotedialer v0.3.0
	github.com/rancher/wharfie v0.5.3
	github.com/rancher/wrangler v1.1.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/rootless-containers/rootlesskit v1.0.1
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.2
	github.com/urfave/cli v1.22.12
	github.com/vishvananda/netlink v1.2.1-beta.2
	github.com/yl2chen/cidranger v1.0.2
	go.etcd.io/etcd/api/v3 v3.5.7
	go.etcd.io/etcd/client/pkg/v3 v3.5.7
	go.etcd.io/etcd/client/v3 v3.5.7
	go.etcd.io/etcd/etcdutl/v3 v3.5.7
	go.etcd.io/etcd/server/v3 v3.5.7
	go.uber.org/zap v1.24.0
	golang.org/x/crypto v0.8.0
	golang.org/x/net v0.9.0
	golang.org/x/sync v0.1.0
	golang.org/x/sys v0.7.0
	google.golang.org/grpc v1.53.0
	gopkg.in/yaml.v2 v2.4.0
	inet.af/tcpproxy v0.0.0-20200125044825-b6bb9b5b8252
	k8s.io/api v0.27.2
	k8s.io/apimachinery v0.27.2
	k8s.io/apiserver v0.27.2
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cloud-provider v0.27.2
	k8s.io/cluster-bootstrap v0.0.0
	k8s.io/component-base v0.27.2
	k8s.io/component-helpers v0.27.2
	k8s.io/cri-api v0.27.2
	k8s.io/klog/v2 v2.90.1
	k8s.io/kubectl v0.25.0
	k8s.io/kubernetes v1.27.2
	k8s.io/utils v0.0.0-20230220204549-a5ecb0141aa5
	sigs.k8s.io/yaml v1.3.0
)

require (
	cloud.google.com/go v0.97.0 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20230106234847-43070de90fa1 // indirect
	github.com/AdamKorcz/go-118-fuzz-build v0.0.0-20221215162035-5330a85ea652 // indirect
	github.com/Azure/azure-sdk-for-go v55.0.0+incompatible // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.27 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.20 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/mocks v0.4.2 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/GoogleCloudPlatform/k8s-cloud-provider v1.18.1-0.20220218231025-f11817397a1b // indirect
	github.com/JeffAshton/win_pdh v0.0.0-20161109143554-76bb4ee9f0ab // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Microsoft/hcsshim v0.10.0-rc.8 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/Rican7/retry v0.1.0 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr v1.4.10 // indirect
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e // indirect
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a // indirect
	github.com/avast/retry-go/v4 v4.3.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/bronze1man/goStrongswanVici v0.0.0-20201105010758-936f38b697fd // indirect
	github.com/canonical/go-dqlite v1.5.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/checkpoint-restore/go-criu/v5 v5.3.0 // indirect
	github.com/cilium/ebpf v0.9.1 // indirect
	github.com/container-orchestrated-devices/container-device-interface v0.5.4 // indirect
	github.com/container-storage-interface/spec v1.7.0 // indirect
	github.com/containerd/btrfs/v2 v2.0.0 // indirect
	github.com/containerd/cgroups/v3 v3.0.1 // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/go-cni v1.1.9 // indirect
	github.com/containerd/go-runc v1.0.0 // indirect
	github.com/containerd/imgcrypt v1.1.7 // indirect
	github.com/containerd/nri v0.3.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/containerd/ttrpc v1.2.2 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/containerd/typeurl/v2 v2.1.1 // indirect
	github.com/containernetworking/cni v1.1.2 // indirect
	github.com/containernetworking/plugins v1.2.0 // indirect
	github.com/containers/ocicrypt v1.1.6 // indirect
	github.com/coreos/go-oidc v2.1.0+incompatible // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/daviddengcn/go-colortext v1.0.0 // indirect
	github.com/docker/cli v23.0.3+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/emicklei/go-restful v2.16.0+incompatible // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/euank/go-kmsg-parser v2.0.0+incompatible // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/exponent-io/jsonpath v0.0.0-20151013193312-d6023ce2651d // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/fvbommel/sortorder v1.0.1 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gofrs/flock v0.8.1 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/cel-go v0.12.6 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/go-containerregistry v0.7.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/hanwen/go-fuse/v2 v2.2.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.2 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/intel/goresctrl v0.3.0 // indirect
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgx/v5 v5.3.1 // indirect
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v0.0.0-20200817173448-b6b71def0850 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/cpuid/v2 v2.1.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/libopenstorage/openstorage v1.0.0 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/lithammer/dedent v1.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mdlayher/genetlink v1.1.0 // indirect
	github.com/mdlayher/netlink v1.4.2 // indirect
	github.com/mdlayher/socket v0.0.0-20211102153432-57e3fa563ecb // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/ipvs v1.1.0 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/signal v0.7.0 // indirect
	github.com/moby/sys/symlink v0.2.0 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mrunalp/fileutils v0.5.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/nats-io/jsm.go v0.0.31-0.20220317133147-fe318f464eee // indirect
	github.com/nats-io/jwt/v2 v2.4.1 // indirect
	github.com/nats-io/nats-server/v2 v2.9.16 // indirect
	github.com/nats-io/nats.go v1.25.0 // indirect
	github.com/nats-io/nkeys v0.4.4 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc2.0.20221005185240-3a7f492d3f1b // indirect
	github.com/opencontainers/runtime-spec v1.1.0-rc.1 // indirect
	github.com/opencontainers/runtime-tools v0.9.1-0.20221107090550-2e043c6bd626 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/rubiojr/go-vhd v0.0.0-20200706105327-02e210299021 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/seccomp/libseccomp-golang v0.9.2-0.20220502022130-f33da4d89646 // indirect
	github.com/shengdoushi/base58 v1.0.0 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spf13/cobra v1.6.1 // indirect
	github.com/stefanberger/go-pkcs11uri v0.0.0-20201008174630-78d3cae3a980 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tchap/go-patricia/v2 v2.3.1 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20220101234140-673ab2c3ae75 // indirect
	github.com/urfave/cli/v2 v2.23.5 // indirect
	github.com/vbatts/tar-split v0.11.2 // indirect
	github.com/vishvananda/netns v0.0.4 // indirect
	github.com/vmware/govmomi v0.30.0 // indirect
	github.com/xiang90/probing v0.0.0-20221125231312-a49e3df8f510 // indirect
	github.com/xlab/treeprint v1.1.0 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.etcd.io/bbolt v1.3.7 // indirect
	go.etcd.io/etcd/client/v2 v2.305.7 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.7 // indirect
	go.etcd.io/etcd/raft/v3 v3.5.7 // indirect
	go.mozilla.org/pkcs7 v0.0.0-20200128120323-432b2356ecb1 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/github.com/emicklei/go-restful/otelrestful v0.35.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.40.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.35.1 // indirect
	go.opentelemetry.io/otel v1.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.14.0 // indirect
	go.opentelemetry.io/otel/metric v0.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.14.0 // indirect
	go.opentelemetry.io/otel/trace v1.14.0 // indirect
	go.opentelemetry.io/proto/otlp v0.19.0 // indirect
	go.starlark.net v0.0.0-20200306205701-8dd3e2ee1dd5 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/oauth2 v0.4.0 // indirect
	golang.org/x/term v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.8.0 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20220117163742-e0b8f11489c5 // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20211230205640-daad0b7ba671 // indirect
	google.golang.org/api v0.60.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230306155012-7f2fa6fef1f4 // indirect
	google.golang.org/protobuf v1.29.1 // indirect
	gopkg.in/gcfg.v1 v1.2.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/warnings.v0 v0.1.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.2.2 // indirect
	k8s.io/apiextensions-apiserver v0.25.4 // indirect
	k8s.io/cli-runtime v0.22.2 // indirect
	k8s.io/code-generator v0.25.4 // indirect
	k8s.io/controller-manager v0.25.4 // indirect
	k8s.io/csi-translation-lib v0.0.0 // indirect
	k8s.io/dynamic-resource-allocation v0.0.0 // indirect
	k8s.io/gengo v0.0.0-20220902162205-c0856e24416d // indirect
	k8s.io/kms v0.0.0 // indirect
	k8s.io/kube-aggregator v0.25.4 // indirect
	k8s.io/kube-controller-manager v0.0.0 // indirect
	k8s.io/kube-openapi v0.0.0-20230501164219-8b0f38b5fd1f // indirect
	k8s.io/kube-proxy v0.0.0 // indirect
	k8s.io/kube-scheduler v0.0.0 // indirect
	k8s.io/kubelet v0.0.0 // indirect
	k8s.io/legacy-cloud-providers v0.0.0 // indirect
	k8s.io/metrics v0.0.0 // indirect
	k8s.io/mount-utils v0.27.2 // indirect
	k8s.io/pod-security-admission v0.0.0 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.1.2 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/kustomize/api v0.13.2 // indirect
	sigs.k8s.io/kustomize/kustomize/v5 v5.0.1 // indirect
	sigs.k8s.io/kustomize/kyaml v0.14.1 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
)
