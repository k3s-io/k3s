module k8s.io/apiserver

require (
	bitbucket.org/ww/goautoneg v0.0.0-20120707110453-75cd24fc2f2c
	github.com/coreos/etcd v3.2.24+incompatible
	github.com/coreos/go-systemd v0.0.0-20161114122254-48702e0da86b
	github.com/coreos/pkg v0.0.0-20160620232715-fa29b1d70f0b
	github.com/docker/docker v0.0.0-20180612054059-a9fbbdc8dd87
	github.com/emicklei/go-restful v0.0.0-20170410110728-ff4f55a20633
	github.com/evanphx/json-patch v0.0.0-20180908160633-36442dbdb585
	github.com/ghodss/yaml v0.0.0-20150909031657-73d445a93680
	github.com/go-openapi/spec v0.0.0-20180213232550-1de3e0542de6
	github.com/gogo/protobuf v0.0.0-20170330071051-c0656edd0d9e
	github.com/golang/glog v0.0.0-20141105023935-44145f04b68c
	github.com/google/gofuzz v0.0.0-20161122191042-44d81051d367
	github.com/grpc-ecosystem/go-grpc-prometheus v0.0.0-20170330212424-2500245aa611
	github.com/hashicorp/golang-lru v0.0.0-20160207214719-a0d98a5f2880
	github.com/pborman/uuid v0.0.0-20150603214016-ca53cad383ca
	github.com/prometheus/client_golang v0.0.0-20170531130054-e7e903064f5e
	github.com/prometheus/client_model v0.0.0-20150212101744-fa8ad6fec335
	github.com/spf13/pflag v1.0.1
	github.com/stretchr/testify v0.0.0-20180319223459-c679ae2cc0cb
	golang.org/x/crypto v0.0.0-20180808211826-de0752318171
	golang.org/x/net v0.0.0-20180906233101-161cd47e91fd
	google.golang.org/grpc v1.7.5
	gopkg.in/natefinch/lumberjack.v2 v2.0.0-20150622162204-20b71e5b60d7
	k8s.io/kube-openapi v0.0.0-20180711000925-0cf8f7e6ed1d
	k8s.io/utils v0.0.0-20180726175726-66066c83e385
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/PuerkitoBio/purell v1.0.0 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20160726150825-5bd2802263f2 // indirect
	github.com/beorn7/perks v0.0.0-20160229213445-3ac7bf7a47d1 // indirect
	github.com/cockroachdb/cmux v0.0.0-20160228191917-112f0506e774 // indirect
	github.com/coreos/bbolt v1.3.1-coreos.6 // indirect
	github.com/coreos/go-semver v0.0.0-20150304020126-568e959cd898 // indirect
	github.com/dgrijalva/jwt-go v0.0.0-20160705203006-01aeca54ebda // indirect
	github.com/go-openapi/jsonpointer v0.0.0-20160704185906-46af16f9f7b1 // indirect
	github.com/go-openapi/jsonreference v0.0.0-20160704190145-13c6e3589ad9 // indirect
	github.com/go-openapi/swag v0.0.0-20170606142751-f3f9494671f9 // indirect
	github.com/google/btree v0.0.0-20160524151835-7d79101e329e // indirect
	github.com/google/go-cmp v0.2.0 // indirect
	github.com/gotestyourself/gotestyourself v2.1.0+incompatible // indirect
	github.com/gregjones/httpcache v0.0.0-20170728041850-787624de3eb7 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.3.0 // indirect
	github.com/imdario/mergo v0.3.5 // indirect
	github.com/jonboulle/clockwork v0.0.0-20141017032234-72f9bd7c4e0c // indirect
	github.com/mailru/easyjson v0.0.0-20170624190925-2f5df55504eb // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/natefinch/lumberjack v2.0.0+incompatible // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.0 // indirect
	github.com/prometheus/common v0.0.0-20170427095455-13ba4ddd0caa // indirect
	github.com/prometheus/procfs v0.0.0-20170519190837-65c1f6f8f0fc // indirect
	github.com/sirupsen/logrus v0.0.0-20170822132746-89742aefa4b2 // indirect
	github.com/ugorji/go v0.0.0-20170107133203-ded73eae5db7 // indirect
	github.com/xiang90/probing v0.0.0-20160813154853-07dd2e8dfe18 // indirect
	golang.org/x/oauth2 v0.0.0-20170412232759-a6bd8cefa181 // indirect
	golang.org/x/time v0.0.0-20161028155119-f51c12702a4d // indirect
	google.golang.org/appengine v1.2.0 // indirect
	google.golang.org/genproto v0.0.0-20170731182057-09f6ed296fc6 // indirect
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/yaml.v1 v1.0.0-20140924161607-9f9df34309c0 // indirect
	gotest.tools v2.1.0+incompatible // indirect
	k8s.io/api v0.0.0-20181009084535-e3c5c37695fb
	k8s.io/apimachinery v0.0.0-20180913025736-6dd46049f395
	k8s.io/client-go v9.0.0+incompatible
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
