package dynamiclistener

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	cert "github.com/rancher/dynamiclistener/cert"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"
)

const (
	httpsMode = "https"
	acmeMode  = "acme"
)

type server struct {
	sync.Mutex

	userConfig          UserConfig
	listenConfigStorage ListenerConfigStorage
	certs               map[string]*tls.Certificate
	ips                 *lru.Cache

	listeners []net.Listener
	servers   []*http.Server

	// dynamic config change on refresh
	activeCert        *tls.Certificate
	activeCA          *x509.Certificate
	activeCAKey       *rsa.PrivateKey
	activeCAKeyString string
	domains           map[string]bool
}

func NewServer(listenConfigStorage ListenerConfigStorage, config UserConfig) (ServerInterface, error) {
	s := &server{
		userConfig:          config,
		listenConfigStorage: listenConfigStorage,
		certs:               map[string]*tls.Certificate{},
	}

	s.ips, _ = lru.New(20)

	if err := s.userConfigure(); err != nil {
		return nil, err
	}

	lc, err := listenConfigStorage.Get()
	if err != nil {
		return nil, err
	}

	return s, s.Update(lc)
}

func (s *server) CACert() (string, error) {
	if s.userConfig.NoCACerts {
		return "", nil
	}
	if s.userConfig.CACerts != "" {
		return s.userConfig.CACerts, nil
	}
	status, err := s.listenConfigStorage.Get()
	if err != nil {
		return "", err
	}

	if status.CACert == "" {
		return "", fmt.Errorf("ca cert not found")
	}

	return status.CACert, nil
}

func (s *server) save() {
	if s.activeCert != nil {
		return
	}

	s.Lock()
	defer s.Unlock()

	changed := false
	cfg, err := s.listenConfigStorage.Get()
	if err != nil {
		return
	}

	if cfg.GeneratedCerts == nil {
		cfg.GeneratedCerts = map[string]string{}
	}

	if cfg.KnownIPs == nil {
		cfg.KnownIPs = map[string]bool{}
	}

	for key, cert := range s.certs {
		certStr := certToString(cert)
		if cfg.GeneratedCerts[key] != certStr {
			cfg.GeneratedCerts[key] = certStr
			changed = true
		}
	}

	for _, obj := range s.ips.Keys() {
		ip, _ := obj.(string)
		if !cfg.KnownIPs[ip] {
			cfg.KnownIPs[ip] = true
			changed = true
		}
	}

	if cfg.CAKey == "" && s.activeCAKey != nil && s.activeCA != nil {
		caCertBuffer := bytes.Buffer{}
		if err := pem.Encode(&caCertBuffer, &pem.Block{
			Type:  cert.CertificateBlockType,
			Bytes: s.activeCA.Raw,
		}); err != nil {
			return
		}

		caKeyBuffer := bytes.Buffer{}
		if err := pem.Encode(&caKeyBuffer, &pem.Block{
			Type:  cert.RSAPrivateKeyBlockType,
			Bytes: x509.MarshalPKCS1PrivateKey(s.activeCAKey),
		}); err != nil {
			return
		}

		cfg.CACert = string(caCertBuffer.Bytes())
		cfg.CAKey = string(caKeyBuffer.Bytes())
		s.activeCAKeyString = cfg.CAKey
		changed = true
	}

	if changed {
		s.listenConfigStorage.Set(cfg)
	}
}

func (s *server) userConfigure() error {
	if s.userConfig.HTTPSPort == 0 {
		s.userConfig.HTTPSPort = 8443
	}

	if s.userConfig.Mode == "" {
		if len(s.userConfig.Domains) > 0 {
			s.userConfig.Mode = acmeMode
		} else {
			s.userConfig.Mode = httpsMode
		}
	}

	s.domains = map[string]bool{}
	for _, d := range s.userConfig.Domains {
		s.domains[d] = true
	}

	if s.userConfig.Key != "" && s.userConfig.Cert != "" {
		cert, err := tls.X509KeyPair([]byte(s.userConfig.Cert), []byte(s.userConfig.Key))
		if err != nil {
			return err
		}
		s.activeCert = &cert
		s.userConfig.Mode = httpsMode
		return s.reload()
	}

	for _, ip := range s.userConfig.KnownIPs {
		netIP := net.ParseIP(ip)
		if netIP != nil {
			s.ips.Add(ip, netIP)
		}
	}
	bindAddress := net.ParseIP(s.userConfig.BindAddress)
	if bindAddress != nil {
		s.ips.Add(s.userConfig.BindAddress, bindAddress)
	}
	return nil
}

func genCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	caKey, err := cert.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	caCert, err := cert.NewSelfSignedCACert(cert.Config{
		CommonName:   "k3s-ca",
		Organization: []string{"k3s-org"},
	}, caKey)
	if err != nil {
		return nil, nil, err
	}

	return caCert, caKey, nil
}

func (s *server) Update(status *ListenerStatus) error {
	s.Lock()
	defer s.getCertificate(&tls.ClientHelloInfo{ServerName: "localhost"})

	if status.CACert != "" && status.CAKey != "" && s.activeCAKeyString != status.CAKey {
		cert, err := tls.X509KeyPair([]byte(status.CACert), []byte(status.CAKey))
		if err != nil {
			s.Unlock()
			return err
		}
		s.activeCAKey = cert.PrivateKey.(*rsa.PrivateKey)
		s.activeCAKeyString = status.CAKey

		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			s.Unlock()
			return err
		}
		s.activeCA = x509Cert
		s.certs = map[string]*tls.Certificate{}
	}

	for ipStr := range status.KnownIPs {
		ip := net.ParseIP(ipStr)
		if len(ip) > 0 {
			s.ips.ContainsOrAdd(ipStr, ip)
		}
	}

	for key, certString := range status.GeneratedCerts {
		cert := stringToCert(certString)
		if cert != nil {
			s.certs[key] = cert
		}
	}

	s.Unlock()
	return s.reload()
}

func (s *server) hostPolicy(ctx context.Context, host string) error {
	s.Lock()
	defer s.Unlock()

	if s.domains[host] {
		return nil
	}

	return errors.New("acme/autocert: host not configured")
}

func (s *server) prompt(tos string) bool {
	return true
}

func (s *server) shutdown() error {
	for _, listener := range s.listeners {
		if err := listener.Close(); err != nil {
			return err
		}
	}
	s.listeners = nil

	for _, server := range s.servers {
		go server.Shutdown(context.Background())
	}
	s.servers = nil

	return nil
}

func (s *server) reload() error {
	if len(s.listeners) > 0 {
		return nil
	}

	if err := s.shutdown(); err != nil {
		return err
	}

	switch s.userConfig.Mode {
	case acmeMode:
		if err := s.serveACME(); err != nil {
			return err
		}
	case httpsMode:
		if err := s.serveHTTPS(); err != nil {
			return err
		}
	}

	return nil
}

func (s *server) ipMapKey() string {
	len := s.ips.Len()
	keys := s.ips.Keys()
	if len == 0 {
		return fmt.Sprintf("local/%d", len)
	} else if len == 1 {
		return fmt.Sprintf("local/%s", keys[0])
	}

	sort.Slice(keys, func(i, j int) bool {
		l, _ := keys[i].(string)
		r, _ := keys[j].(string)
		return l < r
	})
	if len < 6 {
		return fmt.Sprintf("local/%v", keys)
	}

	digest := md5.New()
	for _, k := range keys {
		s, _ := k.(string)
		digest.Write([]byte(s))
	}

	return fmt.Sprintf("local/%v", hex.EncodeToString(digest.Sum(nil)))
}

func (s *server) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.Lock()
	if s.activeCert != nil {
		s.Unlock()
		return s.activeCert, nil
	}

	changed := false
	defer func() {
		if changed {
			s.save()
		}
	}()
	defer s.Unlock()

	mapKey := hello.ServerName
	cn := hello.ServerName
	dnsNames := []string{cn}
	ipBased := false
	var ips []net.IP

	if cn == "" {
		mapKey = s.ipMapKey()
		ipBased = true
	}

	serverNameCert, ok := s.certs[mapKey]
	if ok {
		return serverNameCert, nil
	}

	if ipBased {
		cn = "cattle"
		for _, ipStr := range s.ips.Keys() {
			ip := net.ParseIP(ipStr.(string))
			if len(ip) > 0 {
				ips = append(ips, ip)
			}
		}
	}

	changed = true

	if s.activeCA == nil {
		ca, key, err := genCA()
		if err != nil {
			return nil, err
		}
		s.activeCA = ca
		s.activeCAKey = key
	}

	cfg := cert.Config{
		CommonName:   cn,
		Organization: s.activeCA.Subject.Organization,
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		AltNames: cert.AltNames{
			DNSNames: dnsNames,
			IPs:      ips,
		},
	}

	key, err := cert.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cert, err := cert.NewSignedCert(cfg, key, s.activeCA, s.activeCAKey)
	if err != nil {
		return nil, err
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{
			cert.Raw,
		},
		PrivateKey: key,
	}

	s.certs[mapKey] = tlsCert
	return tlsCert, nil
}

func (s *server) cacheIPHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		h, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			h = req.Host
		}

		ip := net.ParseIP(h)
		if len(ip) > 0 {
			if ok, _ := s.ips.ContainsOrAdd(h, ip); ok {
				go s.save()
			}
		}

		handler.ServeHTTP(resp, req)
	})
}

func (s *server) serveHTTPS() error {
	conf := &tls.Config{
		GetCertificate:           s.getCertificate,
		PreferServerCipherSuites: true,
	}

	listener, err := s.newListener(s.userConfig.BindAddress, s.userConfig.HTTPSPort, conf)
	if err != nil {
		return err
	}

	logger := logrus.StandardLogger()
	server := &http.Server{
		Handler:  s.cacheIPHandler(s.Handler()),
		ErrorLog: log.New(logger.WriterLevel(logrus.DebugLevel), "", log.LstdFlags),
	}

	s.servers = append(s.servers, server)
	s.startServer(listener, server)

	if s.userConfig.HTTPPort > 0 {
		httpListener, err := s.newListener(s.userConfig.BindAddress, s.userConfig.HTTPPort, nil)
		if err != nil {
			return err
		}

		httpServer := &http.Server{
			Handler:  s.cacheIPHandler(httpRedirect(s.Handler())),
			ErrorLog: log.New(logger.WriterLevel(logrus.DebugLevel), "", log.LstdFlags),
		}

		s.servers = append(s.servers, httpServer)
		s.startServer(httpListener, httpServer)
	}

	return nil
}

// Approach taken from letsencrypt, except manglePort is specific to us
func httpRedirect(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(rw http.ResponseWriter, r *http.Request) {
			if r.Header.Get("x-Forwarded-Proto") == "https" ||
				strings.HasPrefix(r.URL.Path, "/ping") ||
				strings.HasPrefix(r.URL.Path, "/health") {
				next.ServeHTTP(rw, r)
				return
			}
			if r.Method != "GET" && r.Method != "HEAD" {
				http.Error(rw, "Use HTTPS", http.StatusBadRequest)
				return
			}
			target := "https://" + manglePort(r.Host) + r.URL.RequestURI()
			http.Redirect(rw, r, target, http.StatusFound)
		})
}

func manglePort(hostport string) string {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return hostport
	}

	portInt = ((portInt / 1000) * 1000) + 443

	return net.JoinHostPort(host, strconv.Itoa(portInt))
}

func (s *server) startServer(listener net.Listener, server *http.Server) {
	go func() {
		if err := server.Serve(listener); err != nil {
			logrus.Errorf("server on %v returned err: %v", listener.Addr(), err)
		}
	}()
}

func (s *server) Handler() http.Handler {
	return s.userConfig.Handler
}

func (s *server) newListener(ip string, port int, config *tls.Config) (net.Listener, error) {
	addr := fmt.Sprintf("%s:%d", ip, port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	l = tcpKeepAliveListener{l.(*net.TCPListener)}

	if config != nil {
		l = tls.NewListener(l, config)
	}

	s.listeners = append(s.listeners, l)
	logrus.Info("Listening on ", addr)
	return l, nil
}

func (s *server) serveACME() error {
	manager := autocert.Manager{
		Cache:      autocert.DirCache("certs-cache"),
		Prompt:     s.prompt,
		HostPolicy: s.hostPolicy,
	}
	conf := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if hello.ServerName == "localhost" || hello.ServerName == "" {
				newHello := *hello
				newHello.ServerName = s.userConfig.Domains[0]
				return manager.GetCertificate(&newHello)
			}
			return manager.GetCertificate(hello)
		},
		NextProtos: []string{"h2", "http/1.1"},
	}

	if s.userConfig.HTTPPort > 0 {
		httpListener, err := s.newListener(s.userConfig.BindAddress, s.userConfig.HTTPPort, nil)
		if err != nil {
			return err
		}

		httpServer := &http.Server{
			Handler:  manager.HTTPHandler(nil),
			ErrorLog: log.New(logrus.StandardLogger().Writer(), "", log.LstdFlags),
		}
		s.servers = append(s.servers, httpServer)
		go func() {
			if err := httpServer.Serve(httpListener); err != nil {
				logrus.Errorf("http server returned err: %v", err)
			}
		}()

	}

	httpsListener, err := s.newListener(s.userConfig.BindAddress, s.userConfig.HTTPSPort, conf)
	if err != nil {
		return err
	}

	httpsServer := &http.Server{
		Handler:  s.Handler(),
		ErrorLog: log.New(logrus.StandardLogger().Writer(), "", log.LstdFlags),
	}
	s.servers = append(s.servers, httpsServer)
	go func() {
		if err := httpsServer.Serve(httpsListener); err != nil {
			logrus.Errorf("https server returned err: %v", err)
		}
	}()

	return nil
}

func stringToCert(certString string) *tls.Certificate {
	parts := strings.Split(certString, "#")
	if len(parts) != 2 {
		return nil
	}

	cert, key := parts[0], parts[1]
	keyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil
	}

	rsaKey, err := x509.ParsePKCS1PrivateKey(keyBytes)
	if err != nil {
		return nil
	}

	certBytes, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return nil
	}

	return &tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  rsaKey,
	}
}

func certToString(cert *tls.Certificate) string {
	certString := base64.StdEncoding.EncodeToString(cert.Certificate[0])
	keyString := base64.StdEncoding.EncodeToString(x509.MarshalPKCS1PrivateKey(cert.PrivateKey.(*rsa.PrivateKey)))
	return certString + "#" + keyString
}

type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}
