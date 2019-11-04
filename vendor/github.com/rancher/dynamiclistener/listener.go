package dynamiclistener

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"sync"

	"github.com/rancher/dynamiclistener/factory"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

type TLSStorage interface {
	Get() (*v1.Secret, error)
	Update(secret *v1.Secret) error
}

type Config struct {
	CN           string
	Organization []string
	TLSConfig    tls.Config
	SANs         []string
}

func NewListener(l net.Listener, storage TLSStorage, caCert *x509.Certificate, caKey crypto.Signer, config Config) (net.Listener, http.Handler, error) {
	if config.CN == "" {
		config.CN = "dynamic"
	}
	if len(config.Organization) == 0 {
		config.Organization = []string{"dynamic"}
	}

	dynamicListener := &listener{
		factory: &factory.TLS{
			CACert:       caCert,
			CAKey:        caKey,
			CN:           config.CN,
			Organization: config.Organization,
		},
		Listener:  l,
		storage:   &nonNil{storage: storage},
		sans:      config.SANs,
		tlsConfig: config.TLSConfig,
	}
	dynamicListener.tlsConfig.GetCertificate = dynamicListener.getCertificate

	return tls.NewListener(dynamicListener, &dynamicListener.tlsConfig), dynamicListener.cacheHandler(), nil
}

type listener struct {
	sync.RWMutex
	net.Listener

	factory   *factory.TLS
	storage   TLSStorage
	version   string
	tlsConfig tls.Config
	cert      *tls.Certificate
	sans      []string
}

func (l *listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return conn, err
	}

	addr := conn.RemoteAddr()
	if addr == nil {
		return conn, nil
	}

	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		logrus.Errorf("failed to parse network %s: %v", addr.Network(), err)
		return conn, nil
	}

	return conn, l.updateCert(host)
}

func (l *listener) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName != "" {
		if err := l.updateCert(hello.ServerName); err != nil {
			return nil, err
		}
	}

	return l.loadCert()
}

func (l *listener) updateCert(cn string) error {
	l.RLock()
	defer l.RUnlock()

	secret, err := l.storage.Get()
	if err != nil {
		return err
	}

	if !factory.NeedsUpdate(secret, append(l.sans, cn)...) {
		return nil
	}

	l.RUnlock()
	l.Lock()
	defer l.RLock()
	defer l.Unlock()

	secret, updated, err := l.factory.AddCN(secret, append(l.sans, cn)...)
	if err != nil {
		return err
	}

	if updated {
		if err := l.storage.Update(secret); err != nil {
			return err
		}
		// clear version to force cert reload
		l.version = ""
	}

	return nil
}

func (l *listener) loadCert() (*tls.Certificate, error) {
	l.RLock()
	defer l.RUnlock()

	secret, err := l.storage.Get()
	if err != nil {
		return nil, err
	}
	if l.cert != nil && l.version == secret.ResourceVersion {
		return l.cert, nil
	}

	defer l.RLock()
	l.RUnlock()
	l.Lock()
	defer l.Unlock()

	secret, err = l.storage.Get()
	if err != nil {
		return nil, err
	}
	if l.cert != nil && l.version == secret.ResourceVersion {
		return l.cert, nil
	}

	cert, err := tls.X509KeyPair(secret.Data[v1.TLSCertKey], secret.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return nil, err
	}

	l.cert = &cert
	return l.cert, nil
}

func (l *listener) cacheHandler() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		h, _, err := net.SplitHostPort(req.Host)
		if err != nil {
			h = req.Host
		}

		ip := net.ParseIP(h)
		if len(ip) > 0 {
			l.updateCert(h)
		}
	})
}

type nonNil struct {
	sync.Mutex
	storage TLSStorage
}

func (n *nonNil) Get() (*v1.Secret, error) {
	n.Lock()
	defer n.Unlock()

	s, err := n.storage.Get()
	if err != nil || s == nil {
		return &v1.Secret{}, err
	}
	return s, nil
}

func (n *nonNil) Update(secret *v1.Secret) error {
	n.Lock()
	defer n.Unlock()

	return n.storage.Update(secret)
}
