package dynamiclistener

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	cert "github.com/rancher/dynamiclistener/cert"
	"github.com/sirupsen/logrus"
)

type server struct {
	sync.Mutex

	userConfig          UserConfig
	listenConfigStorage ListenerConfigStorage
	tlsCert             *tls.Certificate

	ips     map[string]bool
	domains map[string]bool
	cn      string

	listeners []net.Listener
	servers   []*http.Server

	activeCA    *x509.Certificate
	activeCAKey crypto.Signer
}

func NewServer(listenConfigStorage ListenerConfigStorage, config UserConfig) (ServerInterface, error) {
	s := &server{
		userConfig:          config,
		listenConfigStorage: listenConfigStorage,
		cn:                  "cattle",
	}

	s.ips = map[string]bool{}
	s.domains = map[string]bool{}

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
	return "", fmt.Errorf("ca cert not found")
}

func marshalPrivateKey(privateKey crypto.Signer) (string, []byte, error) {
	var (
		keyType string
		bytes   []byte
		err     error
	)
	if key, ok := privateKey.(*ecdsa.PrivateKey); ok {
		keyType = cert.ECPrivateKeyBlockType
		bytes, err = x509.MarshalECPrivateKey(key)
	} else if key, ok := privateKey.(*rsa.PrivateKey); ok {
		keyType = cert.RSAPrivateKeyBlockType
		bytes = x509.MarshalPKCS1PrivateKey(key)
	} else {
		keyType = cert.PrivateKeyBlockType
		bytes, err = x509.MarshalPKCS8PrivateKey(privateKey)
	}
	if err != nil {
		logrus.Errorf("Unable to marshal private key: %v", err)
	}
	return keyType, bytes, err
}

func newPrivateKey() (crypto.Signer, error) {
	caKeyBytes, err := cert.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return nil, err
	}
	caKeyIFace, err := cert.ParsePrivateKeyPEM(caKeyBytes)
	if err != nil {
		return nil, err
	}
	return caKeyIFace.(crypto.Signer), nil
}

func (s *server) save() (_err error) {
	defer func() {
		if _err != nil {
			logrus.Errorf("Saving cert error: %s", _err)
		}
	}()

	certStr, err := certToString(s.tlsCert)
	if err != nil {
		return err
	}
	cfg, err := s.listenConfigStorage.Get()
	if err != nil {
		return err
	}
	cfg.GeneratedCerts = map[string]string{s.cn: certStr}

	_, err = s.listenConfigStorage.Set(cfg)
	return err
}

func (s *server) userConfigure() error {
	if s.userConfig.HTTPSPort == 0 {
		s.userConfig.HTTPSPort = 8443
	}

	for _, d := range s.userConfig.Domains {
		s.domains[d] = true
	}

	for _, ip := range s.userConfig.KnownIPs {
		if netIP := net.ParseIP(ip); netIP != nil {
			s.ips[ip] = true
		}
	}

	if bindAddress := net.ParseIP(s.userConfig.BindAddress); bindAddress != nil {
		s.ips[s.userConfig.BindAddress] = true
	}

	if s.activeCA == nil && s.activeCAKey == nil {
		if s.userConfig.CACerts != "" && s.userConfig.CAKey != "" {
			ca, err := cert.ParseCertsPEM([]byte(s.userConfig.CACerts))
			if err != nil {
				return err
			}
			key, err := cert.ParsePrivateKeyPEM([]byte(s.userConfig.CAKey))
			if err != nil {
				return err
			}
			s.activeCA = ca[0]
			s.activeCAKey = key.(crypto.Signer)
		} else {
			ca, key, err := genCA()
			if err != nil {
				return err
			}
			s.activeCA = ca
			s.activeCAKey = key
		}
	}

	return nil
}

func genCA() (*x509.Certificate, crypto.Signer, error) {
	caKey, err := newPrivateKey()
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

func (s *server) Update(status *ListenerStatus) (_err error) {
	s.Lock()
	defer func() {
		s.Unlock()
		if _err != nil {
			logrus.Errorf("Update cert error: %s", _err)
		}
		if s.tlsCert == nil {
			s.getCertificate(&tls.ClientHelloInfo{ServerName: "localhost"})
		}
	}()

	certString := status.GeneratedCerts[s.cn]
	tlsCert, err := stringToCert(certString)
	if err != nil {
		logrus.Errorf("Update cert unable to convert string to cert: %s", err)
		s.tlsCert = nil
	}
	if tlsCert != nil {
		s.tlsCert = tlsCert
		for i, certBytes := range tlsCert.Certificate {
			cert, err := x509.ParseCertificate(certBytes)
			if err != nil {
				logrus.Errorf("Update cert %d parse error: %s", i, err)
				s.tlsCert = nil
				break
			}

			ips := map[string]bool{}
			for _, ip := range cert.IPAddresses {
				ips[ip.String()] = true
			}

			domains := map[string]bool{}
			for _, domain := range cert.DNSNames {
				domains[domain] = true
			}

			if !(reflect.DeepEqual(ips, s.ips) && reflect.DeepEqual(domains, s.domains)) {
				subset := true
				for ip := range s.ips {
					if !ips[ip] {
						subset = false
						break
					}
				}
				if subset {
					for domain := range s.domains {
						if !domains[domain] {
							subset = false
							break
						}
					}
				}
				if !subset {
					s.tlsCert = nil
				}
				for ip := range ips {
					s.ips[ip] = true
				}
				for domain := range domains {
					s.domains[domain] = true
				}
			}
		}
	}

	return s.reload()
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

	if err := s.serveHTTPS(); err != nil {
		return err
	}

	return nil
}

func (s *server) getCertificate(hello *tls.ClientHelloInfo) (_servingCert *tls.Certificate, _err error) {
	s.Lock()
	changed := false

	defer func() {
		defer s.Unlock()

		if _err != nil {
			logrus.Errorf("Get certificate error: %s", _err)
			return
		}

		if changed {
			s.save()
		}
	}()

	if hello.ServerName != "" && !s.domains[hello.ServerName] {
		s.tlsCert = nil
		s.domains[hello.ServerName] = true
	}

	if s.tlsCert != nil {
		return s.tlsCert, nil
	}

	ips := []net.IP{}
	for ipStr := range s.ips {
		if ip := net.ParseIP(ipStr); ip != nil {
			ips = append(ips, ip)
		}
	}

	dnsNames := []string{}
	for domain := range s.domains {
		dnsNames = append(dnsNames, domain)
	}

	cfg := cert.Config{
		CommonName:   s.cn,
		Organization: s.activeCA.Subject.Organization,
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		AltNames: cert.AltNames{
			DNSNames: dnsNames,
			IPs:      ips,
		},
	}

	key, err := newPrivateKey()
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

	changed = true
	s.tlsCert = tlsCert
	return tlsCert, nil
}

func (s *server) cacheHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		h, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			h = req.Host
		}

		s.Lock()
		if ip := net.ParseIP(h); ip != nil {
			if !s.ips[h] {
				s.ips[h] = true
				s.tlsCert = nil
			}
		} else {
			if !s.domains[h] {
				s.domains[h] = true
				s.tlsCert = nil
			}
		}
		s.Unlock()

		handler.ServeHTTP(resp, req)
	})
}

func (s *server) serveHTTPS() error {
	conf := &tls.Config{
		ClientAuth:               tls.RequestClientCert,
		GetCertificate:           s.getCertificate,
		PreferServerCipherSuites: true,
	}

	listener, err := s.newListener(s.userConfig.BindAddress, s.userConfig.HTTPSPort, conf)
	if err != nil {
		return err
	}

	logger := logrus.StandardLogger()
	server := &http.Server{
		Handler:  s.cacheHandler(s.Handler()),
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
			Handler:  s.cacheHandler(httpRedirect(s.Handler())),
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

func stringToCert(certString string) (*tls.Certificate, error) {
	parts := strings.Split(certString, "#")
	if len(parts) != 2 {
		return nil, errors.New("Unable to split cert into two parts")
	}

	certPart, keyPart := parts[0], parts[1]
	keyBytes, err := base64.StdEncoding.DecodeString(keyPart)
	if err != nil {
		return nil, err
	}

	key, err := cert.ParsePrivateKeyPEM(keyBytes)
	if err != nil {
		return nil, err
	}

	certBytes, err := base64.StdEncoding.DecodeString(certPart)
	if err != nil {
		return nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  key,
	}, nil
}

func certToString(cert *tls.Certificate) (string, error) {
	keyType, keyBytes, err := marshalPrivateKey(cert.PrivateKey.(crypto.Signer))
	if err != nil {
		return "", err
	}

	privateKeyPemBlock := &pem.Block{
		Type:  keyType,
		Bytes: keyBytes,
	}
	pemBytes := pem.EncodeToMemory(privateKeyPemBlock)

	certString := base64.StdEncoding.EncodeToString(cert.Certificate[0])
	keyString := base64.StdEncoding.EncodeToString(pemBytes)
	return certString + "#" + keyString, nil
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
