package dynamiclistener

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

type TLSStorage interface {
	Get() (*v1.Secret, error)
	Update(secret *v1.Secret) error
}

type TLSFactory interface {
	Renew(secret *v1.Secret) (*v1.Secret, error)
	AddCN(secret *v1.Secret, cn ...string) (*v1.Secret, bool, error)
	Merge(target *v1.Secret, additional *v1.Secret) (*v1.Secret, bool, error)
	Filter(cn ...string) []string
	Regenerate(secret *v1.Secret) (*v1.Secret, error)
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

	if config.RegenerateCerts != nil && config.RegenerateCerts() {
		if err := dynamicListener.regenerateCerts(); err != nil {
			return nil, nil, err
		}
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
	RegenerateCerts       func() bool
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
		// busy-wait for certificate preload to complete
		for l.cert == nil {
			runtime.Gosched()
		}

		for {
			wait := 6 * time.Hour
			if err := l.checkExpiration(days); err != nil {
				logrus.Errorf("failed to check and renew dynamic cert: %v", err)
				// Don't go into short retry loop if we're using a static (user-provided) cert.
				// We will still check and print an error every six hours until the user updates the secret with
				// a cert that is not about to expire. Hopefully this will prompt them to take action.
				if err != cert.ErrStaticCert {
					wait = 5 * time.Minute
				}
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

// regenerateCerts regenerates the used certificates and
// updates the secret.
func (l *listener) regenerateCerts() error {
	l.Lock()
	defer l.Unlock()

	secret, err := l.storage.Get()
	if err != nil {
		return err
	}

	newSecret, err := l.factory.Regenerate(secret)
	if err != nil {
		return err
	}
	if err := l.storage.Update(newSecret); err != nil {
		return err
	}
	// clear version to force cert reload
	l.version = ""

	return nil
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

	keyPair, err := tls.X509KeyPair(secret.Data[v1.TLSCertKey], secret.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return err
	}

	certParsed, err := x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return err
	}

	if cert.IsCertExpired(certParsed, days) {
		secret, err := l.factory.Renew(secret)
		if err != nil {
			return err
		}
		if err := l.storage.Update(secret); err != nil {
			return err
		}
		// clear version to force cert reload
		l.version = ""
	}

	return nil
}

func (l *listener) Accept() (net.Conn, error) {
	l.init.Do(func() {
		if len(l.sans) > 0 {
			if err := l.updateCert(l.sans...); err != nil {
				logrus.Errorf("failed to update cert with configured SANs: %v", err)
				return
			}
			if _, err := l.loadCert(nil); err != nil {
				logrus.Errorf("failed to preload certificate: %v", err)
			}
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
			logrus.Errorf("failed to update cert with listener address: %v", err)
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
	id    int
	l     *listener
	ready bool
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
	newConn := hello.Conn
	if hello.ServerName != "" {
		if err := l.updateCert(hello.ServerName); err != nil {
			logrus.Errorf("failed to update cert with TLS ServerName: %v", err)
			return nil, err
		}
	}

	return l.loadCert(newConn)
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

	if factory.IsStatic(secret) || !factory.NeedsUpdate(l.maxSANs, secret, cn...) {
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
		// Clear version to force cert reload next time loadCert is called by TLSConfig's
		// GetCertificate hook to provide a certificate for a new connection. Note that this
		// means the old certificate stays in l.cert until a new connection is made.
		l.version = ""
	}

	return nil
}

func (l *listener) loadCert(currentConn net.Conn) (*tls.Certificate, error) {
	l.RLock()
	defer l.RUnlock()

	secret, err := l.storage.Get()
	if err != nil {
		return nil, err
	}
	if l.cert != nil && l.version == secret.ResourceVersion && secret.ResourceVersion != "" {
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
	if l.cert != nil && l.version == secret.ResourceVersion && secret.ResourceVersion != "" {
		return l.cert, nil
	}

	cert, err := tls.X509KeyPair(secret.Data[v1.TLSCertKey], secret.Data[v1.TLSPrivateKeyKey])
	if err != nil {
		return nil, err
	}

	// cert has changed, close closeWrapper wrapped connections if this isn't the first load
	if currentConn != nil && l.conns != nil && l.cert != nil {
		l.connLock.Lock()
		for _, conn := range l.conns {
			// Don't close a connection that's in the middle of completing a TLS handshake
			if !conn.ready {
				continue
			}
			_ = conn.close()
		}
		l.conns[currentConn.(*closeWrapper).id].ready = true
		l.connLock.Unlock()
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

			if err := l.updateCert(h); err != nil {
				logrus.Errorf("failed to update cert with HTTP request Host header: %v", err)
			}
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
