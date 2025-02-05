//go:build linux
// +build linux

package clientaccess

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_UnitTrustedCA confirms that tokens are validated when the server uses a cert (self-signed or otherwise)
// that is trusted by the OS CA bundle. This test must be run first, since it mucks with the system root certs.
// NOTE:
// This tests only works on Linux, where we can override the default CA bundle with the SSL_CERT_FILE env var.
// On other operating systems, the default CA bundle is loaded via OS-specific crypto APIs.
func Test_UnitTrustedCA(t *testing.T) {
	assert := assert.New(t)
	server := newTLSServer(t, defaultUsername, defaultPassword, false)
	defer server.Close()
	digest, _ := hashCA(getServerCA(server))

	testInfo := &Info{
		CACerts:  getServerCA(server),
		BaseURL:  server.URL,
		Username: defaultUsername,
		Password: defaultPassword,
		caHash:   digest,
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

		info, err = ParseAndValidateToken(server.URL, testCase.token, WithUser("agent"))
		if assert.NoError(err, testCase) {
			assert.Nil(info.CACerts, testCase)
			assert.Equal("agent", info.Username, testCase)
		}
	}
}
