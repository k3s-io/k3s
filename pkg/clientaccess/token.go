package clientaccess

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/kubeadm"
	"github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
)

const (
	tokenPrefix  = "K10"
	caHashLength = sha256.Size * 2

	defaultClientTimeout = 10 * time.Second
)

var (
	defaultClient = &http.Client{
		Timeout: defaultClientTimeout,
	}
	insecureClient = &http.Client{
		Timeout: defaultClientTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
)

// ClientOption is a callback to mutate the http client prior to use
type ClientOption func(*http.Client)

// RequestOption is a callback to mutate the http request prior to use
type RequestOption func(*http.Request)

// Info contains fields that track parsed parts of a cluster join token
type Info struct {
	*kubeadm.BootstrapTokenString

	CACerts  []byte
	BaseURL  string
	Username string
	Password string
	CertFile string
	KeyFile  string
	caHash   string
}

// ValidationOption is a callback to mutate the token prior to use
type ValidationOption func(*Info)

// WithClientCertificate configures certs and keys to be used
// to authenticate the request.
func WithClientCertificate(certFile, keyFile string) ValidationOption {
	return func(i *Info) {
		i.CertFile = certFile
		i.KeyFile = keyFile
	}
}

// WithUser overrides the username from the token with the provided value.
func WithUser(username string) ValidationOption {
	return func(i *Info) {
		i.Username = username
	}
}

// String returns the token data in K10 format
func (i *Info) String() string {
	creds := i.Username + ":" + i.Password
	if i.BootstrapTokenString != nil {
		creds = i.BootstrapTokenString.String()
	}
	digest, _ := hashCA(i.CACerts)
	return tokenPrefix + digest + "::" + creds
}

// Token returns the bootstrap token string, if available.
func (i *Info) Token() string {
	if i.BootstrapTokenString != nil {
		return i.BootstrapTokenString.String()
	}
	return ""
}

// ParseAndValidateToken parses a token, downloads and validates the server's CA bundle,
// and validates it according to the caHash from the token if set.
func ParseAndValidateToken(server string, token string, options ...ValidationOption) (*Info, error) {
	info, err := parseToken(token)
	if err != nil {
		return nil, err
	}

	for _, option := range options {
		option(info)
	}

	if err := info.setAndValidateServer(server); err != nil {
		return nil, err
	}

	return info, nil
}

// setAndValidateServer updates the remote server's cert info, and validates it against the provided hash
func (i *Info) setAndValidateServer(server string) error {
	if err := i.setServer(server); err != nil {
		return err
	}
	return i.validateCAHash()
}

// validateCACerts returns a boolean indicating whether or not a CA bundle matches the
// provided hash, and a string containing the hash of the CA bundle.
func validateCACerts(cacerts []byte, hash string) (bool, string) {
	newHash, _ := hashCA(cacerts)
	return hash == newHash, newHash
}

// hashCA returns the hex-encoded SHA256 digest of a CA bundle.
// If the certificate bundle contains only a single certificate, a legacy hash is generated from
// the literal bytes of the file; usually a PEM-encoded self-signed cluster CA certificate.
// If the certificate bundle contains more than one certificate, the hash is instead generated
// from the DER-encoded root certificate in the bundle. This allows for rotating or renewing the
// cluster CA, as long as the root CA remains the same.
func hashCA(b []byte) (string, error) {
	certs, err := certutil.ParseCertsPEM(b)
	if err != nil {
		return "", err
	}

	if len(certs) > 1 {
		// Bundle contains more than one cert; find the root for the first cert in the bundle and
		// hash the DER of this, instead of just hashing the raw bytes of the whole file.
		roots := x509.NewCertPool()
		intermediates := x509.NewCertPool()
		for i, cert := range certs {
			if i > 0 {
				if len(cert.AuthorityKeyId) == 0 || bytes.Equal(cert.AuthorityKeyId, cert.SubjectKeyId) {
					roots.AddCert(cert)
				} else {
					intermediates.AddCert(cert)
				}
			}
		}
		if chains, err := certs[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates}); err == nil {
			// It's possible but unlikely that there could be multiple valid chains back to a root
			// certificate. Just use the first.
			chain := chains[0]
			b = chain[len(chain)-1].Raw
		}
	}

	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:]), nil
}

// ParseUsernamePassword returns the username and password portion of a token string,
// along with a bool indicating if the token was successfully parsed.
// Kubeadm-style tokens have ID/Secret not Username/Password and therefore will return false (invalid).
func ParseUsernamePassword(token string) (string, string, bool) {
	info, err := parseToken(token)
	if err != nil {
		return "", "", false
	}
	if info.BootstrapTokenString != nil {
		return "", "", false
	}
	return info.Username, info.Password, true
}

// parseToken parses a token into an Info struct
func parseToken(token string) (*Info, error) {
	var info Info

	if len(token) == 0 {
		return nil, errors.New("token must not be empty")
	}

	// Turn bare password or bootstrap token into full K10 token with empty CA hash,
	// for consistent parsing in the section below.
	if !strings.HasPrefix(token, tokenPrefix) {
		_, err := kubeadm.NewBootstrapTokenString(token)
		if err != nil {
			token = tokenPrefix + ":::" + token
		} else {
			token = tokenPrefix + "::" + token
		}
	}

	// Strip off the prefix.
	token = token[len(tokenPrefix):]

	// Split into CA hash and creds.
	parts := strings.SplitN(token, "::", 2)
	token = parts[0]
	if len(parts) > 1 {
		hashLen := len(parts[0])
		if hashLen > 0 && hashLen != caHashLength {
			return nil, errors.New("invalid token CA hash length")
		}
		info.caHash = parts[0]
		token = parts[1]
	}

	// Try to parse creds as bootstrap token string; fall back to basic auth.
	// If neither works, error.
	bts, err := kubeadm.NewBootstrapTokenString(token)
	if err != nil {
		parts = strings.SplitN(token, ":", 2)
		if len(parts) != 2 || len(parts[1]) == 0 {
			return nil, errors.New("invalid token format")
		}
		info.Username = parts[0]
		info.Password = parts[1]
	} else {
		info.BootstrapTokenString = bts
	}

	return &info, nil
}

// GetHTTPClient returns a http client that validates TLS server certificates using the provided CA bundle.
// If the CA bundle is empty, it validates using the default http client using the OS CA bundle.
// If the CA bundle is not empty but does not contain any valid certs, it validates using
// an empty CA bundle (which will always fail).
// If valid cert+key paths can be loaded from the provided paths, they are used for client cert auth.
func GetHTTPClient(cacerts []byte, certFile, keyFile string, options ...any) *http.Client {
	if len(cacerts) == 0 {
		return defaultClient
	}

	tlsConfig := &tls.Config{
		RootCAs: x509.NewCertPool(),
	}

	tlsConfig.RootCAs.AppendCertsFromPEM(cacerts)

	// Try to load certs from the provided cert and key. We ignore errors,
	// as it is OK if the paths were empty or the files don't currently exist.
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err == nil {
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	client := &http.Client{
		Timeout: defaultClientTimeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig:   tlsConfig,
		},
	}

	for _, o := range options {
		if clientOption, ok := o.(ClientOption); ok {
			clientOption(client)
		}
	}
	return client
}

func WithTimeout(d time.Duration) ClientOption {
	return func(c *http.Client) {
		c.Timeout = d
		c.Transport.(*http.Transport).ResponseHeaderTimeout = d
	}
}

func WithHeader(k, v string) RequestOption {
	return func(r *http.Request) {
		r.Header.Add(k, v)
	}
}

// Get makes a request to a subpath of info's BaseURL
func (i *Info) Get(path string, options ...any) ([]byte, error) {
	u, err := url.Parse(i.BaseURL)
	if err != nil {
		return nil, err
	}
	p, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	p.Scheme = u.Scheme
	p.Host = u.Host
	client := GetHTTPClient(i.CACerts, i.CertFile, i.KeyFile, options...)
	return get(p.String(), client, i.Username, i.Password, i.Token(), options...)
}

// Put makes a request to a subpath of info's BaseURL
func (i *Info) Put(path string, body []byte, options ...any) error {
	u, err := url.Parse(i.BaseURL)
	if err != nil {
		return err
	}
	p, err := url.Parse(path)
	if err != nil {
		return err
	}
	p.Scheme = u.Scheme
	p.Host = u.Host
	client := GetHTTPClient(i.CACerts, i.CertFile, i.KeyFile, options...)
	return put(p.String(), body, client, i.Username, i.Password, i.Token(), options...)
}

// Post makes a request to a subpath of info's BaseURL
func (i *Info) Post(path string, body []byte, options ...any) ([]byte, error) {
	u, err := url.Parse(i.BaseURL)
	if err != nil {
		return nil, err
	}
	p, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	p.Scheme = u.Scheme
	p.Host = u.Host
	client := GetHTTPClient(i.CACerts, i.CertFile, i.KeyFile, options...)
	return post(p.String(), body, client, i.Username, i.Password, i.Token(), options...)
}

// setServer sets the BaseURL and CACerts fields of the Info by connecting to the server
// and storing the CA bundle.
func (i *Info) setServer(server string) error {
	url, err := url.Parse(server)
	if err != nil {
		return errors.Wrapf(err, "Invalid server url, failed to parse: %s", server)
	}

	if url.Scheme != "https" {
		return errors.New("only https:// URLs are supported, invalid scheme: " + server)
	}

	for strings.HasSuffix(url.Path, "/") {
		url.Path = url.Path[:len(url.Path)-1]
	}

	cacerts, err := getCACerts(*url)
	if err != nil {
		return err
	}

	i.BaseURL = url.String()
	i.CACerts = cacerts
	return nil
}

// ValidateCAHash validates that info's caHash matches the CACerts hash.
func (i *Info) validateCAHash() error {
	if len(i.caHash) > 0 && len(i.CACerts) == 0 {
		// Warn if the user provided a CA hash but we're not going to validate because it's already trusted
		logrus.Warn("Cluster CA certificate is trusted by the host CA bundle. " +
			"Token CA hash will not be validated.")
	} else if len(i.caHash) == 0 && len(i.CACerts) > 0 {
		// Warn if the CA is self-signed but the user didn't provide a hash to validate it against
		logrus.Warn("Cluster CA certificate is not trusted by the host CA bundle, but the token does not include a CA hash. " +
			"Use the full token from the server's node-token file to enable Cluster CA validation.")
	} else if len(i.CACerts) > 0 && len(i.caHash) > 0 {
		// only verify CA hash if the server cert is not trusted by the OS CA bundle
		if ok, serverHash := validateCACerts(i.CACerts, i.caHash); !ok {
			return fmt.Errorf("token CA hash does not match the Cluster CA certificate hash: %s != %s", i.caHash, serverHash)
		}
	}
	return nil
}

// getCACerts retrieves the CA bundle from a server.
// An error is raised if the CA bundle cannot be retrieved,
// or if the server's cert is not signed by the returned bundle.
func getCACerts(u url.URL) ([]byte, error) {
	u.Path = "/cacerts"
	url := u.String()

	// This first request is expected to fail. If the server has
	// a cert that can be validated using the default CA bundle, return
	// success with no CA certs.
	_, err := get(url, defaultClient, "", "", "")
	if err == nil {
		return nil, nil
	}

	// Download the CA bundle using a client that does not validate certs.
	cacerts, err := get(url, insecureClient, "", "", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get CA certs")
	}

	// Request the CA bundle again, validating that the CA bundle can be loaded
	// and used to validate the server certificate. This should only fail if we somehow
	// get an empty CA bundle. or if the dynamiclistener cert is incorrectly signed.
	_, err = get(url, GetHTTPClient(cacerts, "", ""), "", "", "")
	if err != nil {
		return nil, errors.Wrap(err, "CA cert validation failed")
	}

	return cacerts, nil
}

// get makes a request to a url using a provided client and credentials,
// returning the response body.
func get(u string, client *http.Client, username, password, token string, options ...any) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	} else if username != "" {
		req.SetBasicAuth(username, password)
	}

	for _, o := range options {
		if requestOption, ok := o.(RequestOption); ok {
			requestOption(req)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return readBody(resp)
}

// put makes a request to a url using a provided client and credentials,
// only an error is returned
func put(u string, body []byte, client *http.Client, username, password, token string, options ...any) error {
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	} else if username != "" {
		req.SetBasicAuth(username, password)
	}

	for _, o := range options {
		if requestOption, ok := o.(RequestOption); ok {
			requestOption(req)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	_, err = readBody(resp)
	return err
}

// post makes a request to a url using a provided client and credentials,
// returning the response body and error.
func post(u string, body []byte, client *http.Client, username, password, token string, options ...any) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	} else if username != "" {
		req.SetBasicAuth(username, password)
	}

	for _, o := range options {
		if requestOption, ok := o.(RequestOption); ok {
			requestOption(req)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return readBody(resp)
}

// readBody attempts to get the body from the response. If the response status
// code is not in the 2XX range, an error is returned. An attempt is made to
// decode the error body as a metav1.Status and return a StatusError, if
// possible.
func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	warnings, _ := net.ParseWarningHeaders(resp.Header["Warning"])
	for _, warning := range warnings {
		if warning.Code == 299 && len(warning.Text) != 0 {
			logrus.Warnf(warning.Text)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		status := metav1.Status{}
		if err := json.Unmarshal(b, &status); err == nil && status.Kind == "Status" {
			return nil, &apierrors.StatusError{ErrStatus: status}
		}
		return nil, fmt.Errorf("%s: %s", resp.Request.URL, resp.Status)
	}
	return b, nil
}

// FormatToken takes a username:password string or join token, and a path to a certificate bundle, and
// returns a string containing the full K10 format token string. If the credentials are
// empty, an empty token is returned. If the certificate bundle does not exist or does not
// contain a valid bundle, an error is returned.
func FormatToken(creds, certFile string) (string, error) {
	if len(creds) == 0 {
		return "", nil
	}

	b, err := os.ReadFile(certFile)
	if err != nil {
		return "", err
	}
	return FormatTokenBytes(creds, b)
}

// FormatTokenBytes has the same interface as FormatToken, but accepts a byte slice instead
// of file path.
func FormatTokenBytes(creds string, b []byte) (string, error) {
	if len(creds) == 0 {
		return "", nil
	}

	digest, err := hashCA(b)
	if err != nil {
		return "", err
	}

	return tokenPrefix + digest + "::" + creds, nil
}
