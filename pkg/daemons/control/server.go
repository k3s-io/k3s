package control

import (
	"bufio"
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/daemons/config"
	certutil "github.com/rancher/norman/pkg/cert"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	sapp "k8s.io/kubernetes/cmd/kube-scheduler/app"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/proxy/util"
	_ "k8s.io/kubernetes/pkg/util/reflector/prometheus" // for reflector metric registration
	_ "k8s.io/kubernetes/pkg/util/workqueue/prometheus" // for workqueue metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration
)

var (
	localhostIP        = net.ParseIP("127.0.0.1")
	x509KeyServerOnly  = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	x509KeyClientUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	requestHeaderCN    = "system:auth-proxy"
	kubeconfigTemplate = template.Must(template.New("kubeconfig").Parse(`apiVersion: v1
clusters:
- cluster:
    server: {{.URL}}
    certificate-authority-data: {{.CACert}}
  name: local
contexts:
- context:
    cluster: local
    namespace: default
    user: user
  name: Default
current-context: Default
kind: Config
preferences: {}
users:
- name: user
  user:
    username: {{.User}}
    password: {{.Password}}
`))
)

func Server(ctx context.Context, cfg *config.Control) error {
	rand.Seed(time.Now().UTC().UnixNano())

	runtime := &config.ControlRuntime{}
	cfg.Runtime = runtime

	if err := prepare(cfg, runtime); err != nil {
		return err
	}

	cfg.Runtime.Tunnel = setupTunnel()
	util.DisableProxyHostnameCheck = true

	auth, handler, err := apiServer(ctx, cfg, runtime)
	if err != nil {
		return err
	}

	runtime.Handler = handler
	runtime.Authenticator = auth

	if !cfg.NoScheduler {
		scheduler(cfg, runtime)
	}

	controllerManager(cfg, runtime)

	return nil
}

func controllerManager(cfg *config.Control, runtime *config.ControlRuntime) {
	argsMap := map[string]string{
		"kubeconfig":                       runtime.KubeConfigSystem,
		"service-account-private-key-file": runtime.ServiceKey,
		"allocate-node-cidrs":              "true",
		"cluster-cidr":                     cfg.ClusterIPRange.String(),
		"root-ca-file":                     runtime.ServerCA,
		"port":                             "10252",
		"bind-address":                     "127.0.0.1",
		"secure-port":                      "0",
	}
	if cfg.NoLeaderElect {
		argsMap["leader-elect"] = "false"
	}

	args := config.GetArgsList(argsMap, cfg.ExtraControllerArgs)

	command := cmapp.NewControllerManagerCommand()
	command.SetArgs(args)

	go func() {
		logrus.Infof("Running kube-controller-manager %s", config.ArgString(args))
		logrus.Fatalf("controller-manager exited: %v", command.Execute())
	}()
}

func scheduler(cfg *config.Control, runtime *config.ControlRuntime) {
	argsMap := map[string]string{
		"kubeconfig":   runtime.KubeConfigSystem,
		"port":         "10251",
		"bind-address": "127.0.0.1",
		"secure-port":  "0",
	}
	if cfg.NoLeaderElect {
		argsMap["leader-elect"] = "false"
	}
	args := config.GetArgsList(argsMap, cfg.ExtraSchedulerAPIArgs)

	command := sapp.NewSchedulerCommand()
	command.SetArgs(args)

	go func() {
		logrus.Infof("Running kube-scheduler %s", config.ArgString(args))
		logrus.Fatalf("scheduler exited: %v", command.Execute())
	}()
}

func apiServer(ctx context.Context, cfg *config.Control, runtime *config.ControlRuntime) (authenticator.Request, http.Handler, error) {
	argsMap := make(map[string]string)
	if len(cfg.ETCDEndpoints) > 0 {
		argsMap["storage-backend"] = "etcd3"
		argsMap["etcd-servers"] = strings.Join(cfg.ETCDEndpoints, ",")
		if cfg.ETCDKeyFile != "" {
			argsMap["etcd-keyfile"] = cfg.ETCDKeyFile
		}
		if cfg.ETCDCAFile != "" {
			argsMap["etcd-cafile"] = cfg.ETCDCAFile
		}
		if cfg.ETCDCertFile != "" {
			argsMap["etcd-certfile"] = cfg.ETCDCertFile
		}
	}

	certDir := filepath.Join(cfg.DataDir, "tls/temporary-certs")
	os.MkdirAll(certDir, 0700)

	// TODO: sqlite doesn't need the watch cache, but etcd does, so make this dynamic
	argsMap["watch-cache"] = "false"
	argsMap["cert-dir"] = certDir
	argsMap["allow-privileged"] = "true"
	argsMap["authorization-mode"] = strings.Join([]string{modes.ModeNode, modes.ModeRBAC}, ",")
	argsMap["service-account-signing-key-file"] = runtime.ServiceKey
	argsMap["service-cluster-ip-range"] = cfg.ServiceIPRange.String()
	argsMap["advertise-port"] = strconv.Itoa(cfg.AdvertisePort)
	argsMap["advertise-address"] = localhostIP.String()
	argsMap["insecure-port"] = "0"
	argsMap["secure-port"] = strconv.Itoa(cfg.ListenPort)
	argsMap["bind-address"] = localhostIP.String()
	argsMap["tls-cert-file"] = runtime.ServingKubeAPICert
	argsMap["tls-private-key-file"] = runtime.ServingKubeAPIKey
	argsMap["service-account-key-file"] = runtime.ServiceKey
	argsMap["service-account-issuer"] = "k3s"
	argsMap["api-audiences"] = "unknown"
	argsMap["basic-auth-file"] = runtime.PasswdFile
	argsMap["kubelet-client-certificate"] = runtime.ClientKubeAPICert
	argsMap["kubelet-client-key"] = runtime.ClientKubeAPIKey
	argsMap["client-ca-file"] = runtime.ClientCA
	argsMap["requestheader-client-ca-file"] = runtime.RequestHeaderCA
	argsMap["requestheader-allowed-names"] = requestHeaderCN
	argsMap["proxy-client-key-file"] = runtime.ClientAuthProxyKey
	argsMap["proxy-client-cert-file"] = runtime.ClientAuthProxyCert
	argsMap["requestheader-extra-headers-prefix"] = "X-Remote-Extra-"
	argsMap["requestheader-group-headers"] = "X-Remote-Group"
	argsMap["requestheader-username-headers"] = "X-Remote-User"
	argsMap["enable-admission-plugins"] = "ServiceAccount,NamespaceLifecycle,LimitRanger,DefaultStorageClass,ResourceQuota,DefaultTolerationSeconds"

	args := config.GetArgsList(argsMap, cfg.ExtraAPIArgs)

	command := app.NewAPIServerCommand(ctx.Done())
	command.SetArgs(args)

	go func() {
		logrus.Infof("Running kube-apiserver %s", config.ArgString(args))
		logrus.Fatalf("apiserver exited: %v", command.Execute())
	}()

	startupConfig := <-app.StartupConfig

	return startupConfig.Authenticator, startupConfig.Handler, nil
}

func defaults(config *config.Control) {
	if config.ClusterIPRange == nil {
		_, clusterIPNet, _ := net.ParseCIDR("10.42.0.0/16")
		config.ClusterIPRange = clusterIPNet
	}

	if config.ServiceIPRange == nil {
		_, serviceIPNet, _ := net.ParseCIDR("10.43.0.0/16")
		config.ServiceIPRange = serviceIPNet
	}

	if len(config.ClusterDNS) == 0 {
		config.ClusterDNS = net.ParseIP("10.43.0.10")
	}

	if config.AdvertisePort == 0 {
		config.AdvertisePort = 6445
	}

	if config.ListenPort == 0 {
		config.ListenPort = 6444
	}

	if config.DataDir == "" {
		config.DataDir = "./management-state"
	}
}

func prepare(config *config.Control, runtime *config.ControlRuntime) error {
	var err error

	defaults(config)

	if _, err := os.Stat(config.DataDir); os.IsNotExist(err) {
		if err := os.MkdirAll(config.DataDir, 0700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	config.DataDir, err = filepath.Abs(config.DataDir)
	if err != nil {
		return err
	}

	os.MkdirAll(path.Join(config.DataDir, "tls"), 0700)
	os.MkdirAll(path.Join(config.DataDir, "cred"), 0700)

	runtime.ServerCA = path.Join(config.DataDir, "tls", "server-ca.crt")
	runtime.ServerCAKey = path.Join(config.DataDir, "tls", "server-ca.key")
	runtime.ClientCA = path.Join(config.DataDir, "tls", "client-ca.crt")
	runtime.ClientCAKey = path.Join(config.DataDir, "tls", "client-ca.key")
	runtime.RequestHeaderCA = path.Join(config.DataDir, "tls", "request-header-ca.crt")
	runtime.RequestHeaderCAKey = path.Join(config.DataDir, "tls", "request-header-ca.key")
	runtime.ServingKubeAPICert = path.Join(config.DataDir, "tls", "serving-kube-apiserver.crt")
	runtime.ServingKubeAPIKey = path.Join(config.DataDir, "tls", "serving-kube-apiserver.key")
	runtime.ClientKubeAPIKey = path.Join(config.DataDir, "tls", "client-kube-apiserver.key")
	runtime.ClientKubeAPICert = path.Join(config.DataDir, "tls", "client-kube-apiserver.crt")
	runtime.ClientAuthProxyKey = path.Join(config.DataDir, "tls", "client-auth-proxy.key")
	runtime.ClientAuthProxyCert = path.Join(config.DataDir, "tls", "client-auth-proxy.crt")
	runtime.ServiceKey = path.Join(config.DataDir, "tls", "service.key")
	runtime.PasswdFile = path.Join(config.DataDir, "cred", "passwd")
	runtime.KubeConfigSystem = path.Join(config.DataDir, "cred", "kubeconfig-system.yaml")

	if err := genCerts(config, runtime); err != nil {
		return err
	}

	if err := genServiceAccount(runtime); err != nil {
		return err
	}

	if err := genUsers(config, runtime); err != nil {
		return err
	}

	return readTokens(runtime)
}

func readTokens(runtime *config.ControlRuntime) error {
	f, err := os.Open(runtime.PasswdFile)
	if err != nil {
		return err
	}
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(record) < 2 {
			continue
		}

		switch record[1] {
		case "node":
			runtime.NodeToken = "node:" + record[0]
		case "admin":
			runtime.ClientToken = "admin:" + record[0]
		}
	}

	return nil
}

func ensureNodeToken(config *config.Control, runtime *config.ControlRuntime) error {
	if config.ClusterSecret == "" {
		return nil
	}

	f, err := os.Open(runtime.PasswdFile)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := &strings.Builder{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		if parts[1] == "node" {
			if parts[0] == config.ClusterSecret {
				return nil
			}
			parts[0] = config.ClusterSecret
			line = strings.Join(parts, ",")
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}

	if scan.Err() != nil {
		return scan.Err()
	}

	f.Close()
	return ioutil.WriteFile(runtime.PasswdFile, []byte(buf.String()), 0600)
}

func genUsers(config *config.Control, runtime *config.ControlRuntime) error {
	if s, err := os.Stat(runtime.PasswdFile); err == nil && s.Size() > 0 {
		return ensureNodeToken(config, runtime)
	}

	adminToken, err := getToken()
	if err != nil {
		return err
	}
	systemToken, err := getToken()
	if err != nil {
		return err
	}
	nodeToken, err := getToken()
	if err != nil {
		return err
	}
	schedulerToken, err := getToken()
	if err != nil {
		return err
	}
	controllerToken, err := getToken()
	if err != nil {
		return err
	}
	proxyToken, err := getToken()
	if err != nil {
		return err
	}

	if config.ClusterSecret != "" {
		nodeToken = config.ClusterSecret
	}

	passwd := fmt.Sprintf(`%s,admin,system:admin,"system:masters"
%s,system,system,"system:masters"
%s,node,system:node,"system:masters"
%s,scheduler,system:kube-scheduler,""
%s,controller,system:kube-controller-manager,""
%s,kube-proxy,system:kube-proxy,"system:nodes"
`, adminToken, systemToken, nodeToken, schedulerToken, controllerToken, proxyToken)

	caCertBytes, err := ioutil.ReadFile(runtime.ServerCA)
	if err != nil {
		return err
	}

	caCert := base64.StdEncoding.EncodeToString(caCertBytes)

	if err := kubeConfig(runtime.KubeConfigSystem, fmt.Sprintf("https://localhost:%d", config.ListenPort), caCert,
		"system", systemToken); err != nil {
		return err
	}

	return ioutil.WriteFile(runtime.PasswdFile, []byte(passwd), 0600)
}

func getToken() (string, error) {
	token := make([]byte, 16, 16)
	_, err := cryptorand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

func genCerts(config *config.Control, runtime *config.ControlRuntime) error {
	if err := genServerCerts(config, runtime); err != nil {
		return err
	}
	if err := genClientCerts(config, runtime); err != nil {
		return err
	}
	if err := genRequestHeaderCerts(config, runtime); err != nil {
		return err
	}
	return nil
}

func genServerCerts(config *config.Control, runtime *config.ControlRuntime) error {
	regen, err := createSigningCertKey("k3s-server", runtime.ServerCA, runtime.ServerCAKey)
	if err != nil {
		return err
	}

	_, apiServerServiceIP, err := master.DefaultServiceIPRange(*config.ServiceIPRange)
	if err != nil {
		return err
	}

	if err := createClientCertKey(regen, "kube-apiserver",
		nil, &certutil.AltNames{
			DNSNames: []string{"kubernetes.default.svc", "kubernetes.default", "kubernetes", "localhost"},
			IPs:      []net.IP{apiServerServiceIP, localhostIP},
		}, x509KeyServerOnly,
		runtime.ServerCA, runtime.ServerCAKey,
		runtime.ServingKubeAPICert, runtime.ServingKubeAPIKey); err != nil {
		return err
	}

	return nil
}

func genClientCerts(config *config.Control, runtime *config.ControlRuntime) error {
	regen, err := createSigningCertKey("k3s-client", runtime.ClientCA, runtime.ClientCAKey)
	if err != nil {
		return err
	}

	// _, apiServerServiceIP, err := master.DefaultServiceIPRange(*config.ServiceIPRange)
	// if err != nil {
	// 	return err
	// }

	if err := createClientCertKey(regen, "kube-apiserver",
		nil, nil, x509KeyClientUsage,
		runtime.ClientCA, runtime.ClientCAKey,
		runtime.ClientKubeAPICert, runtime.ClientKubeAPIKey); err != nil {
		return err
	}

	return nil
}

func genRequestHeaderCerts(config *config.Control, runtime *config.ControlRuntime) error {
	regen, err := createSigningCertKey("k3s-request-header", runtime.RequestHeaderCA, runtime.RequestHeaderCAKey)
	if err != nil {
		return err
	}

	if err := createClientCertKey(regen, requestHeaderCN,
		nil, nil, x509KeyClientUsage,
		runtime.RequestHeaderCA, runtime.RequestHeaderCAKey,
		runtime.ClientAuthProxyCert, runtime.ClientAuthProxyKey); err != nil {
		return err
	}

	return nil
}

func createClientCertKey(regen bool, commonName string, organization []string, altNames *certutil.AltNames, extKeyUsage []x509.ExtKeyUsage, caCertFile, caKeyFile, certFile, keyFile string) error {
	if !regen {
		if exists(certFile, keyFile) {
			return nil
		}
	}

	caKeyBytes, err := ioutil.ReadFile(caKeyFile)
	if err != nil {
		return err
	}

	caBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return err
	}

	caKey, err := certutil.ParsePrivateKeyPEM(caKeyBytes)
	if err != nil {
		return err
	}

	caCert, err := certutil.ParseCertsPEM(caBytes)
	if err != nil {
		return err
	}

	key, err := certutil.NewPrivateKey()
	if err != nil {
		return err
	}

	cfg := certutil.Config{
		CommonName:   commonName,
		Organization: organization,
		Usages:       extKeyUsage,
	}
	if altNames != nil {
		cfg.AltNames = *altNames
	}
	cert, err := certutil.NewSignedCert(cfg, key, caCert[0], caKey.(*rsa.PrivateKey))
	if err != nil {
		return err
	}

	if err := certutil.WriteKey(keyFile, certutil.EncodePrivateKeyPEM(key)); err != nil {
		return err
	}

	return certutil.WriteCert(certFile, append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(caCert[0])...))
}

func exists(files ...string) bool {
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			return false
		}
	}
	return true
}

func genServiceAccount(runtime *config.ControlRuntime) error {
	_, keyErr := os.Stat(runtime.ServiceKey)
	if keyErr == nil {
		return nil
	}

	key, err := certutil.NewPrivateKey()
	if err != nil {
		return err
	}

	return certutil.WriteKey(runtime.ServiceKey, certutil.EncodePrivateKeyPEM(key))
}

func createSigningCertKey(prefix, certFile, keyFile string) (bool, error) {
	if exists(certFile, keyFile) {
		return false, nil
	}

	caKey, err := certutil.NewPrivateKey()
	if err != nil {
		return false, err
	}

	cfg := certutil.Config{
		CommonName: fmt.Sprintf("%s-ca@%d", prefix, time.Now().Unix()),
	}

	cert, err := certutil.NewSelfSignedCACert(cfg, caKey)
	if err != nil {
		return false, err
	}

	if err := certutil.WriteKey(keyFile, certutil.EncodePrivateKeyPEM(caKey)); err != nil {
		return false, err
	}

	if err := certutil.WriteCert(certFile, certutil.EncodeCertPEM(cert)); err != nil {
		return false, err
	}
	return true, nil
}

func kubeConfig(dest, url, cert, user, password string) error {
	data := struct {
		URL      string
		CACert   string
		User     string
		Password string
	}{
		URL:      url,
		CACert:   cert,
		User:     user,
		Password: password,
	}

	output, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer output.Close()

	return kubeconfigTemplate.Execute(output, &data)
}
