package clientaccess

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/stretchr/testify/assert"
)

var (
	defaultUsername = "server"
	defaultPassword = "token"
)

// Test_UnitTrustedCA confirms that tokens are validated when the server uses a cert (self-signed or otherwise)
// that is trusted by the OS CA bundle. This test must be run first, since it mucks with the system root certs.
func Test_UnitTrustedCA(t *testing.T) {
	assert := assert.New(t)
	server := newTLSServer(t, defaultUsername, defaultPassword, false)
	defer server.Close()

	testInfo := &Info{
		CACerts:  getServerCA(server),
		BaseURL:  server.URL,
		Username: defaultUsername,
		Password: defaultPassword,
		caHash:   hashCA(getServerCA(server)),
	}

	testCases := []struct {
		token    string
		expected string
	}{
		{defaultPassword, ""},
		{testInfo.String(), testInfo.Username},
	}

	// Point OS CA bundle at this test's CA cert to simulate a trusted CA cert.
	// Note that this only works if the OS CA bundle has not yet been loaded in this process,
	// as it is cached for the duration of the process lifetime.
	// Ref: https://github.com/golang/go/issues/41888
	path := t.TempDir() + "/ca.crt"
	writeServerCA(server, path)
	os.Setenv("SSL_CERT_FILE", path)

	for _, testCase := range testCases {
		info, err := ParseAndValidateToken(server.URL, testCase.token)
		if assert.NoError(err, testCase) {
			assert.Nil(info.CACerts, testCase)
			assert.Equal(testCase.expected, info.Username, testCase.token)
		}

		info, err = ParseAndValidateTokenForUser(server.URL, testCase.token, "agent")
		if assert.NoError(err, testCase) {
			assert.Nil(info.CACerts, testCase)
			assert.Equal("agent", info.Username, testCase)
		}
	}

	// Confirm that the cert is actually trusted by the OS CA bundle by making a request
	// with empty cert pool
	testInfo.CACerts = nil
	res, err := testInfo.Get("/v1-k3s/server-bootstrap")
	assert.NoError(err)
	assert.NotEmpty(res)
}

// Test_UnitUntrustedCA confirms that tokens are validated when the server uses a self-signed cert
// that is NOT trusted by the OS CA bundle.
func Test_UnitUntrustedCA(t *testing.T) {
	assert := assert.New(t)
	server := newTLSServer(t, defaultUsername, defaultPassword, false)
	defer server.Close()

	testInfo := &Info{
		CACerts:  getServerCA(server),
		BaseURL:  server.URL,
		Username: defaultUsername,
		Password: defaultPassword,
		caHash:   hashCA(getServerCA(server)),
	}

	testCases := []struct {
		token    string
		expected string
	}{
		{defaultPassword, ""},
		{testInfo.String(), testInfo.Username},
	}

	for _, testCase := range testCases {
		info, err := ParseAndValidateToken(server.URL, testCase.token)
		if assert.NoError(err, testCase) {
			assert.Equal(testInfo.CACerts, info.CACerts, testCase)
			assert.Equal(testCase.expected, info.Username, testCase)
		}

		info, err = ParseAndValidateTokenForUser(server.URL, testCase.token, "agent")
		if assert.NoError(err, testCase) {
			assert.Equal(testInfo.CACerts, info.CACerts, testCase)
			assert.Equal("agent", info.Username, testCase)
		}
	}
}

// Test_UnitInvalidServers tests that invalid server URLs are properly rejected
func Test_UnitInvalidServers(t *testing.T) {
	assert := assert.New(t)
	testCases := []struct {
		server   string
		token    string
		expected string
	}{
		{" https://localhost:6443", "token", "Invalid server url, failed to parse:  https://localhost:6443: parse \" https://localhost:6443\": first path segment in URL cannot contain colon"},
		{"http://localhost:6443", "token", "only https:// URLs are supported, invalid scheme: http://localhost:6443"},
	}

	for _, testCase := range testCases {
		_, err := ParseAndValidateToken(testCase.server, testCase.token)
		assert.EqualError(err, testCase.expected, testCase)

		_, err = ParseAndValidateTokenForUser(testCase.server, testCase.token, defaultUsername)
		assert.EqualError(err, testCase.expected, testCase)
	}
}

// Test_UnitInvalidTokens tests that tokens which are empty, invalid, or incorrect are properly rejected
func Test_UnitInvalidTokens(t *testing.T) {
	assert := assert.New(t)
	server := newTLSServer(t, defaultUsername, defaultPassword, false)
	defer server.Close()

	testCases := []struct {
		server   string
		token    string
		expected string
	}{
		{server.URL, "", "token must not be empty"},
		{server.URL, "K10::", "invalid token format"},
		{server.URL, "K10::x", "invalid token format"},
		{server.URL, "K10::x:", "invalid token format"},
		{server.URL, "K10XX::x:y", "invalid token CA hash length"},
		{server.URL,
			"K10XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX::x:y",
			"token CA hash does not match the Cluster CA certificate hash: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX != " + hashCA(getServerCA(server))},
	}

	for _, testCase := range testCases {
		info, err := ParseAndValidateToken(testCase.server, testCase.token)
		assert.EqualError(err, testCase.expected, testCase)
		assert.Nil(info, testCase)

		info, err = ParseAndValidateTokenForUser(testCase.server, testCase.token, defaultUsername)
		assert.EqualError(err, testCase.expected, testCase)
		assert.Nil(info, testCase)
	}
}

// Test_UnitInvalidCredentials tests that tokens which don't have valid credentials are rejected
func Test_UnitInvalidCredentials(t *testing.T) {
	assert := assert.New(t)
	server := newTLSServer(t, defaultUsername, defaultPassword, false)
	defer server.Close()

	testInfo := &Info{
		CACerts:  getServerCA(server),
		BaseURL:  server.URL,
		Username: "nobody",
		Password: "invalid",
		caHash:   hashCA(getServerCA(server)),
	}

	testCases := []string{
		testInfo.Password,
		testInfo.String(),
	}

	for _, testCase := range testCases {
		info, err := ParseAndValidateToken(server.URL, testCase)
		assert.NoError(err, testCase)
		if assert.NotNil(info) {
			res, err := info.Get("/v1-k3s/server-bootstrap")
			assert.Error(err, testCase)
			assert.Empty(res, testCase)
		}

		info, err = ParseAndValidateTokenForUser(server.URL, testCase, defaultUsername)
		assert.NoError(err, testCase)
		if assert.NotNil(info) {
			res, err := info.Get("/v1-k3s/server-bootstrap")
			assert.Error(err, testCase)
			assert.Empty(res, testCase)
		}
	}
}

// Test_UnitWrongCert tests that errors are returned when the server's cert isn't issued by its CA
func Test_UnitWrongCert(t *testing.T) {
	assert := assert.New(t)
	server := newTLSServer(t, defaultUsername, defaultPassword, true)
	defer server.Close()

	info, err := ParseAndValidateToken(server.URL, defaultPassword)
	assert.Error(err)
	assert.Nil(info)

	info, err = ParseAndValidateTokenForUser(server.URL, defaultPassword, defaultUsername)
	assert.Error(err)
	assert.Nil(info)
}

// Test_UnitConnectionFailures tests that connections are timed out properly
func Test_UnitConnectionFailures(t *testing.T) {
	testDuration := (defaultClientTimeout * 2) + time.Second
	assert := assert.New(t)
	testCases := []struct {
		server string
		token  string
	}{
		{"https://192.0.2.1:6443", "token"}, // RFC 5735 TEST-NET-1 for use in documentation and example code
		{"https://localhost:1", "token"},
	}

	for _, testCase := range testCases {
		startTime := time.Now()
		info, err := ParseAndValidateToken(testCase.server, testCase.token)
		assert.Error(err, testCase)
		assert.Nil(info, testCase)
		assert.WithinDuration(time.Now(), startTime, testDuration, testCase)

		startTime = time.Now()
		info, err = ParseAndValidateTokenForUser(testCase.server, testCase.token, defaultUsername)
		assert.Error(err, testCase)
		assert.Nil(info, testCase)
		assert.WithinDuration(startTime, time.Now(), testDuration, testCase)
	}
}

// Test_UnitUserPass tests that usernames and passwords are parsed or not parsed from token strings
func Test_UnitUserPass(t *testing.T) {
	assert := assert.New(t)
	testCases := []struct {
		token    string
		username string
		password string
		expect   bool
	}{
		{"K10XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX::username:password", "username", "password", true},
		{"password", "", "password", true},
		{"K10X::x", "", "", false},
	}

	for _, testCase := range testCases {
		username, password, ok := ParseUsernamePassword(testCase.token)
		assert.Equal(testCase.expect, ok, testCase)
		if ok {
			assert.Equal(testCase.username, username, testCase)
			assert.Equal(testCase.password, password, testCase)
		}
	}
}

// Test_UnitParseAndGet tests URL handling along some hard-to-reach code paths
func Test_UnitParseAndGet(t *testing.T) {
	assert := assert.New(t)
	server := newTLSServer(t, defaultUsername, defaultPassword, false)
	defer server.Close()

	testCases := []struct {
		extraBasePre  string
		extraBasePost string
		path          string
		parseFail     bool
		getFail       bool
	}{
		{"/", "", "/cacerts", false, false},
		{"/%2", "", "/cacerts", true, false},
		{"", "", "/%2", false, true},
		{"", "/%2", "/cacerts", false, true},
	}

	for _, testCase := range testCases {
		info, err := ParseAndValidateTokenForUser(server.URL+testCase.extraBasePre, defaultPassword, defaultUsername)
		// Check for expected error when parsing server + token
		if testCase.parseFail {
			assert.Error(err, testCase)
		} else if assert.NoError(err, testCase) {
			info.BaseURL = server.URL + testCase.extraBasePost
			_, err := info.Get(testCase.path)
			// Check for expected error when making Get request
			if testCase.getFail {
				assert.Error(err, testCase)
			} else {
				assert.NoError(err, testCase)
			}
		}
	}
}

// newTLSServer returns a HTTPS server that mocks the basic functionality required to validate K3s join tokens.
// Each call to this function will generate new CA and server certificates unique to the returned server.
func newTLSServer(t *testing.T, username, password string, sendWrongCA bool) *httptest.Server {
	var server *httptest.Server
	server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1-k3s/server-bootstrap" {
			if authUsername, authPassword, ok := r.BasicAuth(); ok != true || authPassword != password || authUsername != username {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			bootstrapData := &config.ControlRuntimeBootstrap{}
			w.Header().Set("Content-Type", "application/json")
			if err := bootstrap.ReadFromDisk(w, bootstrapData); err != nil {
				t.Errorf("failed to write bootstrap: %v", err)
			}
			return
		}

		if r.URL.Path == "/cacerts" {
			w.Header().Set("Content-Type", "text/plain")
			if _, err := w.Write(getServerCA(server)); err != nil {
				t.Errorf("Failed to write cacerts: %v", err)
			}
			return
		}

		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}))

	// Create new CA cert and key
	caCert, caKey, err := factory.GenCA()
	if err != nil {
		t.Fatal(err)
	}

	// Generate new server cert; reuse the key from the CA
	cfg := cert.Config{
		CommonName:   "localhost",
		Organization: []string{"testing"},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		AltNames: cert.AltNames{
			DNSNames: []string{"localhost"},
			IPs:      []net.IP{net.IPv4(127, 0, 0, 1)},
		},
	}
	serverCert, err := cert.NewSignedCert(cfg, caKey, caCert, caKey)
	if err != nil {
		t.Fatal(err)
	}

	// Bind server and CA certs into chain for TLS listener configuration
	server.TLS = &tls.Config{}
	server.TLS.Certificates = []tls.Certificate{
		{Certificate: [][]byte{serverCert.Raw}, Leaf: serverCert, PrivateKey: caKey},
		{Certificate: [][]byte{caCert.Raw}, Leaf: caCert},
	}

	if sendWrongCA {
		// Create new CA cert and key and use that as the CA cert instead of the one that actually signed the server cert
		badCert, _, err := factory.GenCA()
		if err != nil {
			t.Fatal(err)
		}
		server.TLS.Certificates[1].Certificate[0] = badCert.Raw
		server.TLS.Certificates[1].Leaf = badCert
	}

	server.StartTLS()
	return server
}

// getServerCA returns a byte slice containing the PEM encoding of the server's CA certificate
func getServerCA(server *httptest.Server) []byte {
	certLen := len(server.TLS.Certificates)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.TLS.Certificates[certLen-1].Certificate[0]})
}

// writeServerCA writes the PEM-encoded server certificate to a given path
func writeServerCA(server *httptest.Server, path string) error {
	certOut, err := os.Create(path)
	if err != nil {
		return err
	}
	defer certOut.Close()

	if _, err := certOut.Write(getServerCA(server)); err != nil {
		return err
	}

	return nil
}
