package dynamiclistener

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rancher/dynamiclistener/factory"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

type TLSStorage interface {
	Get() (*v1.Secret, error)
	Update(secret *v1.Secret) error
}

type TLSFactory interface {
	Refresh(secret *v1.Secret) (*v1.Secret, error)
	AddCN(secret *v1.Secret, cn ...string) (*v1.Secret, bool, error)
	Merge(target *v1.Secret, additional *v1.Secret) (*v1.Secret, bool, error)
	Filter(cn ...string) []string
}

type SetFactory interface {
	SetFactory(tls TLSFactory)
}

func NewListener(l net.Listener, storage TLSStorage, caCert *x509.Certificate, caKey crypto.Signer, config Config) (net.Listener, http.Handler, error) {
	if config.CN == "" {
		config.CN = "dynamic"
	}
	if len(config.Organization) == 0 {
		config.Organization = []string{"dynamic"}
	}
	if config.TLSConfig == nil {
		config.TLSConfig = &tls.Config{}
	}

	dynamicListener := &listener{
		factory: &factory.TLS{
			CACert:       caCert,
			CAKey:        caKey,
			CN:           config.CN,
			Organization: config.Organization,
			FilterCN:     allowDefaultSANs(config.SANs, config.FilterCN),
		},
		Listener:  l,
		storage:   &nonNil{storage: storage},
		sans:      config.SANs,
		maxSANs:   config.MaxSANs,
		tlsConfig: config.TLSConfig,
	}
	if dynamicListener.tlsConfig == nil {
		dynamicListener.tlsConfig = &tls.Config{}
	}
	dynamicListener.tlsConfig.GetCertificate = dynamicListener.getCertificate

	if config.CloseConnOnCertChange {
		if len(dynamicListener.tlsConfig.Certificates) == 0 {
			dynamicListener.tlsConfig.NextProtos = []string{"http/1.1"}
		}
		dynamicListener.conns = map[int]*closeWrapper{}
	}

	if setter, ok := storage.(SetFactory); ok {
		setter.SetFactory(dynamicListener.factory)
	}

	if config.ExpirationDaysCheck == 0 {
		config.ExpirationDaysCheck = 30
	}

	tlsListener := tls.NewListener(dynamicListener.WrapExpiration(config.ExpirationDaysCheck), dynamicListener.tlsConfig)
	return tlsListener, dynamicListener.cacheHandler(), nil
}

func allowDefaultSANs(sans []string, next func(...string) []string) func(...string) []string {
	if next == nil {
		return nil
	} else if len(sans) == 0 {
		return next
	}

	sanMap := map[string]bool{}
	for _, san := range sans {
		sanMap[san] = true
	}

	return func(s ...string) []string {
		var (
			good    []string
			unknown []string
		)
		for _, s := range s {
			if sanMap[s] {
				good = append(good, s)
			} else {
				unknown = append(unknown, s)
			}
		}

		return append(good, next(unknown...)...)
	}
}

type cancelClose struct {
	cancel func()
	net.Listener
}

func (c *cancelClose) Close() error {
	c.cancel()
	return c.Listener.Close()
}

type Config struct {
	CN                    string
	Organization          []string
	TLSConfig             *tls.Config
	SANs                  []string
	MaxSANs               int
	ExpirationDaysCheck   int
	CloseConnOnCertChange bool
	FilterCN              func(...string) []string
}

type listener struct {
	sync.RWMutex
	net.Listener

	conns    map[int]*closeWrapper
	connID   int
	connLock sync.Mutex

	factory   TLSFactory
	storage   TLSStorage
	version   string
	tlsConfig *tls.Config
	cert      *tls.Certificate
	sans      []string
	maxSANs   int
	init      sync.Once
}

func (l *listener) WrapExpiration(days int) net.Listener {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Minute)

		for {
			wait := 6 * time.Hour
			if err := l.checkExpiration(days); err != nil {
				logrus.Errorf("failed to check and refresh dynamic cert: %v", err)
				wait = 5 + time.Minute
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()

	return &cancelClose{
		cancel:   cancel,
		Listener: l,
	}
}

func (l *listener) checkExpiration(days int) error {
	l.Lock()
	defer l.Unlock()

	if days == 0 {
		return nil
	}

	if l.cert == nil {
		return nil
	}

	secret, err := l.storage.Get()
	if err != nil {
		return err
	}

	cert, err := tls.X509KeyPair(secret.Data[v1.TLSCertKey], secret.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return err
	}

	certParsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return err
	}

	if time.Now().UTC().Add(time.Hour * 24 * time.Duration(days)).After(certParsed.NotAfter) {
		secret, err := l.factory.Refresh(secret)
		if err != nil {
			return err
		}
		return l.storage.Update(secret)
	}

	return nil
}

func (l *listener) Accept() (net.Conn, error) {
	l.init.Do(func() {
		if len(l.sans) > 0 {
			l.updateCert(l.sans...)
		}
	})

	conn, err := l.Listener.Accept()
	if err != nil {
		return conn, err
	}

	addr := conn.LocalAddr()
	if addr == nil {
		return conn, nil
	}

	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		logrus.Errorf("failed to parse network %s: %v", addr.Network(), err)
		return conn, nil
	}

	if !strings.Contains(host, ":") {
		if err := l.updateCert(host); err != nil {
			logrus.Infof("failed to create TLS cert for: %s, %v", host, err)
		}
	}

	if l.conns != nil {
		conn = l.wrap(conn)
	}

	return conn, nil
}

func (l *listener) wrap(conn net.Conn) net.Conn {
	l.connLock.Lock()
	defer l.connLock.Unlock()
	l.connID++

	wrapper := &closeWrapper{
		Conn: conn,
		id:   l.connID,
		l:    l,
	}
	l.conns[l.connID] = wrapper

	return wrapper
}

type closeWrapper struct {
	net.Conn
	id int
	l  *listener
}

func (c *closeWrapper) close() error {
	delete(c.l.conns, c.id)
	return c.Conn.Close()
}

func (c *closeWrapper) Close() error {
	c.l.connLock.Lock()
	defer c.l.connLock.Unlock()
	return c.close()
}

func (l *listener) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if hello.ServerName != "" {
		if err := l.updateCert(hello.ServerName); err != nil {
			return nil, err
		}
	}

	return l.loadCert()
}

func (l *listener) updateCert(cn ...string) error {
	cn = l.factory.Filter(cn...)
	if len(cn) == 0 {
		return nil
	}

	l.RLock()
	defer l.RUnlock()

	secret, err := l.storage.Get()
	if err != nil {
		return err
	}

	if !factory.NeedsUpdate(l.maxSANs, secret, cn...) {
		return nil
	}

	l.RUnlock()
	l.Lock()
	defer l.RLock()
	defer l.Unlock()

	secret, updated, err := l.factory.AddCN(secret, append(l.sans, cn...)...)
	if err != nil {
		return err
	}

	if updated {
		if err := l.storage.Update(secret); err != nil {
			return err
		}
		// clear version to force cert reload
		l.version = ""
		if l.conns != nil {
			l.connLock.Lock()
			for _, conn := range l.conns {
				_ = conn.close()
			}
			l.connLock.Unlock()
		}
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
	l.version = secret.ResourceVersion
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
			for _, v := range req.Header["User-Agent"] {
				if strings.Contains(strings.ToLower(v), "mozilla") {
					return
				}
			}

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
