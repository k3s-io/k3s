package server

import (
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
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/cmd/kube-apiserver/app"
	"k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	cmapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	cmoptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	sapp "k8s.io/kubernetes/cmd/kube-scheduler/app"
	"k8s.io/kubernetes/pkg/apis/componentconfig"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	"k8s.io/kubernetes/pkg/controller/nodeipam/ipam"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	"k8s.io/kubernetes/pkg/master"
	_ "k8s.io/kubernetes/pkg/util/reflector/prometheus" // for reflector metric registration
	_ "k8s.io/kubernetes/pkg/util/workqueue/prometheus" // for workqueue metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration
)

var (
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

type ServerConfig struct {
	PublicHostname string
	PublicIP       *net.IP
	PublicPort     int
	ListenAddr     net.IP
	ListenPort     int
	ClusterIPRange net.IPNet
	ServiceIPRange net.IPNet
	DataDir        string
	LeaderElect    bool

	tlsCert          string
	tlsKey           string
	tlsCA            string
	tlsCAKey         string
	tokenCA          string
	tokenCAKey       string
	serviceKey       string
	passwdFile       string
	kubeConfigSystem string
	kubeConfigNode   string

	NodeCert      string
	NodeKey       string
	ClientToken   string
	NodeToken     string
	KubeConfig    string
	Handler       http.Handler
	Authenticator authenticator.Request
}

func Server(ctx context.Context, config *ServerConfig) error {
	rand.Seed(time.Now().UTC().UnixNano())

	auth, handler, err := apiServer(ctx, config)
	if err != nil {
		return err
	}

	config.Handler = handler
	config.Authenticator = auth

	if err := scheduler(ctx, config); err != nil {
		return err
	}

	if err := controllerManager(ctx, config); err != nil {
		return err
	}

	return nil
}

func controllerManager(ctx context.Context, cfg *ServerConfig) error {
	s := cmoptions.NewKubeControllerManagerOptions()
	s.Generic.Kubeconfig = cfg.kubeConfigSystem
	s.Generic.ComponentConfig.LeaderElection.LeaderElect = cfg.LeaderElect
	s.Generic.ComponentConfig.ServiceAccountKeyFile = cfg.serviceKey
	s.Generic.ComponentConfig.AllocateNodeCIDRs = true
	s.Generic.ComponentConfig.ClusterCIDR = cfg.ClusterIPRange.String()
	s.Generic.ComponentConfig.CIDRAllocatorType = string(ipam.RangeAllocatorType)
	s.Generic.ComponentConfig.RootCAFile = cfg.tokenCA

	c, err := s.Config(cmapp.KnownControllers(), cmapp.ControllersDisabledByDefault.List())
	if err != nil {
		return err
	}

	go func() {
		err := cmapp.Run(c.Complete())
		logrus.Fatalf("controller-manager exited: %v", err)
	}()

	return nil
}

func scheduler(ctx context.Context, cfg *ServerConfig) error {
	options, err := sapp.NewOptions()
	if err != nil {
		return err
	}

	config, err := options.ApplyDefaults(&componentconfig.KubeSchedulerConfiguration{})
	if err != nil {
		return err
	}

	config.ClientConnection.KubeConfigFile = cfg.kubeConfigSystem
	s, err := sapp.NewSchedulerServer(config, "")
	if err != nil {
		return err
	}

	go func() {
		logrus.Fatalf("scheduler exited: %v", s.Run(ctx.Done()))
	}()

	return nil
}

func apiServer(ctx context.Context, config *ServerConfig) (authenticator.Request, http.Handler, error) {
	if err := prepare(config); err != nil {
		return nil, nil, err
	}

	s := options.NewServerRunOptions()
	s.InsecureServing.BindPort = 0
	s.AllowPrivileged = true
	s.Authorization.Mode = modes.ModeNode + "," + modes.ModeRBAC // + "," + modes.ModeAlwaysAllow
	s.ServiceAccountSigningKeyFile = config.serviceKey
	s.ServiceClusterIPRange = config.ServiceIPRange
	s.SecureServing.PublicPort = config.PublicPort
	s.SecureServing.PublicIP = config.PublicIP
	s.SecureServing.BindPort = config.ListenPort
	s.SecureServing.BindAddress = config.ListenAddr
	s.SecureServing.ServerCert = apiserveroptions.GeneratableKeyCert{
		CertKey: apiserveroptions.CertKey{
			CertFile: config.tlsCert,
			KeyFile:  config.tlsKey,
		},
	}
	s.Authentication.ServiceAccounts.KeyFiles = []string{config.serviceKey}
	s.Authentication.PasswordFile.BasicAuthFile = config.passwdFile
	s.KubeletConfig.CertFile = config.NodeCert
	s.KubeletConfig.KeyFile = config.NodeKey

	os.Chdir(config.DataDir)

	serverConfig, server, err := app.CreateServerChain(s, ctx.Done())
	if err != nil {
		return nil, nil, err
	}

	prepared := server.PrepareRun()

	go func() {
		err := prepared.Run(ctx.Done())
		logrus.Fatalf("apiserver exited: %v", err)
	}()

	return serverConfig.GenericConfig.Authentication.Authenticator, prepared.Handler, nil
}

func prepare(config *ServerConfig) error {
	var err error

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

	name := config.PublicHostname
	config.tlsCert = path.Join(config.DataDir, "tls", name+".crt")
	config.tlsKey = path.Join(config.DataDir, "tls", name+".key")
	config.tlsCA = path.Join(config.DataDir, "tls", "ca.crt")
	config.tlsCAKey = path.Join(config.DataDir, "tls", "ca.key")
	config.tokenCA = path.Join(config.DataDir, "tls", "token-ca.crt")
	config.tokenCAKey = path.Join(config.DataDir, "tls", "token-ca.key")
	config.serviceKey = path.Join(config.DataDir, "tls", "service.key")
	config.passwdFile = path.Join(config.DataDir, "cred", "passwd")
	config.KubeConfig = path.Join(config.DataDir, "cred", "kubeconfig.yaml")
	config.kubeConfigSystem = path.Join(config.DataDir, "cred", "kubeconfig-system.yaml")
	config.kubeConfigNode = path.Join(config.DataDir, "cred", "kubeconfig-node.yaml")
	config.NodeKey = path.Join(config.DataDir, "tls", "token-node.key")
	config.NodeCert = path.Join(config.DataDir, "tls", "token-node.crt")

	regen := false
	if _, err := os.Stat(config.tlsCA); err != nil {
		regen = true
		if err := genCA(config); err != nil {
			return err
		}
	}

	if err := genServiceAccount(config); err != nil {
		return err
	}

	if err := genTLS(regen, config); err != nil {
		return err
	}

	if err := genTokenTLS(config); err != nil {
		return err
	}

	if err := genUsers(config); err != nil {
		return err
	}

	return readTokens(config)
}

func readTokens(config *ServerConfig) error {
	f, err := os.Open(config.passwdFile)
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
			config.NodeToken = "node:" + record[0]
		case "admin":
			config.ClientToken = "admin:" + record[0]
		}
	}

	return nil
}

func genUsers(config *ServerConfig) error {
	if s, err := os.Stat(config.passwdFile); err == nil && s.Size() > 0 {
		return nil
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

	passwd := fmt.Sprintf(`%s,admin,admin,system:masters
%s,system,system,system:masters
%s,node,node,system:masters
`, adminToken, systemToken, nodeToken)

	caCertBytes, err := ioutil.ReadFile(config.tlsCA)
	if err != nil {
		return err
	}

	caCert := base64.StdEncoding.EncodeToString(caCertBytes)

	if err := kubeConfig(config.KubeConfig, fmt.Sprintf("https://%s:%d", config.PublicHostname, config.ListenPort), caCert,
		"admin", adminToken); err != nil {
		return err
	}

	if err := kubeConfig(config.kubeConfigSystem, fmt.Sprintf("https://localhost:%d", config.ListenPort), caCert,
		"system", systemToken); err != nil {
		return err
	}

	if err := kubeConfig(config.kubeConfigNode, fmt.Sprintf("https://%s:%d", config.PublicHostname, config.ListenPort), caCert,
		"node", nodeToken); err != nil {
		return err
	}

	return ioutil.WriteFile(config.passwdFile, []byte(passwd), 0600)
}

func getToken() (string, error) {
	token := make([]byte, 16, 16)
	_, err := cryptorand.Read(token)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(token), err
}

func genTokenTLS(config *ServerConfig) error {
	regen := false
	if _, err := os.Stat(config.tokenCA); err != nil {
		regen = true
		if err := genTokenCA(config); err != nil {
			return err
		}
	}

	_, apiServerServiceIP, err := master.DefaultServiceIPRange(config.ServiceIPRange)
	if err != nil {
		return err
	}

	cfg := certutil.Config{
		CommonName: "kubernetes",
		AltNames: certutil.AltNames{
			DNSNames: []string{"kubernetes.default.svc", "kubernetes.default", "kubernetes", "localhost"},
			IPs:      []net.IP{net.ParseIP("127.0.0.1"), apiServerServiceIP},
		},
		Usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	if _, err := os.Stat(config.NodeCert); err == nil && !regen {
		return nil
	}

	caKeyBytes, err := ioutil.ReadFile(config.tokenCAKey)
	if err != nil {
		return err
	}

	caBytes, err := ioutil.ReadFile(config.tokenCA)
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

	cert, err := certutil.NewSignedCert(cfg, key, caCert[0], caKey.(*rsa.PrivateKey))
	if err != nil {
		return err
	}

	if err := certutil.WriteKey(config.NodeKey, certutil.EncodePrivateKeyPEM(key)); err != nil {
		return err
	}

	return certutil.WriteCert(config.NodeCert, append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(caCert[0])...))
}

func genTLS(regen bool, config *ServerConfig) error {
	if !regen {
		_, certErr := os.Stat(config.tlsCert)
		_, keyErr := os.Stat(config.tlsKey)
		if certErr == nil && keyErr == nil {
			return nil
		}
	}

	_, apiServerServiceIP, err := master.DefaultServiceIPRange(config.ServiceIPRange)
	if err != nil {
		return err
	}

	cfg := certutil.Config{
		CommonName: config.PublicHostname,
		AltNames: certutil.AltNames{
			DNSNames: []string{"kubernetes.default.svc", "kubernetes.default", "kubernetes", config.PublicHostname},
			IPs:      []net.IP{apiServerServiceIP},
		},
		Usages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	bindIP := config.ListenAddr.String()
	if bindIP == "0.0.0.0" {
		cfg.AltNames.DNSNames = append(cfg.AltNames.DNSNames, "localhost")
	} else {
		cfg.AltNames.IPs = append(cfg.AltNames.IPs, config.ListenAddr)
	}

	caKeyBytes, err := ioutil.ReadFile(config.tlsCAKey)
	if err != nil {
		return err
	}

	caBytes, err := ioutil.ReadFile(config.tlsCA)
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

	cert, err := certutil.NewSignedCert(cfg, key, caCert[0], caKey.(*rsa.PrivateKey))
	if err != nil {
		return err
	}

	if err := certutil.WriteKey(config.tlsKey, certutil.EncodePrivateKeyPEM(key)); err != nil {
		return err
	}

	return certutil.WriteCert(config.tlsCert, append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(caCert[0])...))
}

func genServiceAccount(config *ServerConfig) error {
	_, keyErr := os.Stat(config.serviceKey)
	if keyErr == nil {
		return nil
	}

	key, err := certutil.NewPrivateKey()
	if err != nil {
		return err
	}

	return certutil.WriteKey(config.serviceKey, certutil.EncodePrivateKeyPEM(key))
}

func genTokenCA(config *ServerConfig) error {
	caKey, err := certutil.NewPrivateKey()
	if err != nil {
		return err
	}

	cfg := certutil.Config{
		CommonName: fmt.Sprintf("%s-ca@%d", "k3s-token", time.Now().Unix()),
	}

	cert, err := certutil.NewSelfSignedCACert(cfg, caKey)
	if err != nil {
		return err
	}

	if err := certutil.WriteKey(config.tokenCAKey, certutil.EncodePrivateKeyPEM(caKey)); err != nil {
		return err
	}

	return certutil.WriteCert(config.tokenCA, certutil.EncodeCertPEM(cert))
}

func genCA(config *ServerConfig) error {
	caKey, err := certutil.NewPrivateKey()
	if err != nil {
		return err
	}

	cfg := certutil.Config{
		CommonName: fmt.Sprintf("%s-ca@%d", "k3s", time.Now().Unix()),
	}

	cert, err := certutil.NewSelfSignedCACert(cfg, caKey)
	if err != nil {
		return err
	}

	if err := certutil.WriteKey(config.tlsCAKey, certutil.EncodePrivateKeyPEM(caKey)); err != nil {
		return err
	}

	return certutil.WriteCert(config.tlsCA, certutil.EncodeCertPEM(cert))
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
