package clientaccess

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	insecureClient = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
)

type OverrideURLCallback func(config []byte) (*url.URL, error)

type clientToken struct {
	caHash   string
	username string
	password string
}

func AgentAccessInfoToTempKubeConfig(tempDir, server, token string) (string, error) {
	f, err := ioutil.TempFile(tempDir, "tmp-")
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	err = accessInfoToKubeConfig(f.Name(), server, token)
	if err != nil {
		os.Remove(f.Name())
	}
	return f.Name(), err
}

func AgentAccessInfoToKubeConfig(destFile, server, token string) error {
	return accessInfoToKubeConfig(destFile, server, token)
}

type Info struct {
	URL      string `json:"url,omitempty"`
	CACerts  []byte `json:"cacerts,omitempty"`
	username string
	password string
	Token    string `json:"token,omitempty"`
}

func (i *Info) WriteKubeConfig(destFile string) error {
	return clientcmd.WriteToFile(*i.KubeConfig(), destFile)
}

func (i *Info) KubeConfig() *clientcmdapi.Config {
	config := clientcmdapi.NewConfig()

	cluster := clientcmdapi.NewCluster()
	cluster.CertificateAuthorityData = i.CACerts
	cluster.Server = i.URL

	authInfo := clientcmdapi.NewAuthInfo()
	if i.password != "" {
		authInfo.Username = i.username
		authInfo.Password = i.password
	} else if i.Token != "" {
		if username, pass, ok := ParseUsernamePassword(i.Token); ok {
			authInfo.Username = username
			authInfo.Password = pass
		} else {
			authInfo.Token = i.Token
		}
	}

	context := clientcmdapi.NewContext()
	context.AuthInfo = "default"
	context.Cluster = "default"

	config.Clusters["default"] = cluster
	config.AuthInfos["default"] = authInfo
	config.Contexts["default"] = context
	config.CurrentContext = "default"

	return config
}

func ParseAndValidateToken(server, token string) (*Info, error) {
	url, err := url.Parse(server)
	if err != nil {
		return nil, errors.Wrapf(err, "Invalid url, failed to parse %s", server)
	}

	if url.Scheme != "https" {
		return nil, fmt.Errorf("only https:// URLs are supported, invalid scheme: %s", server)
	}

	for strings.HasSuffix(url.Path, "/") {
		url.Path = url.Path[:len(url.Path)-1]
	}

	parsedToken, err := parseToken(token)
	if err != nil {
		return nil, err
	}

	cacerts, err := GetCACerts(*url)
	if err != nil {
		return nil, err
	}

	if len(cacerts) > 0 && len(parsedToken.caHash) > 0 {
		if ok, hash, newHash := validateCACerts(cacerts, parsedToken.caHash); !ok {
			return nil, fmt.Errorf("token does not match the server %s != %s", hash, newHash)
		}
	}

	if err := validateToken(*url, cacerts, parsedToken.username, parsedToken.password); err != nil {
		return nil, err
	}

	return &Info{
		URL:      url.String(),
		CACerts:  cacerts,
		username: parsedToken.username,
		password: parsedToken.password,
		Token:    token,
	}, nil
}

func accessInfoToKubeConfig(destFile, server, token string) error {
	info, err := ParseAndValidateToken(server, token)
	if err != nil {
		return err
	}

	return info.WriteKubeConfig(destFile)
}

func validateToken(u url.URL, cacerts []byte, username, password string) error {
	u.Path = "/apis"
	_, err := get(u.String(), GetHTTPClient(cacerts), username, password)
	if err != nil {
		return errors.Wrap(err, "token is not valid")
	}
	return nil
}

func validateCACerts(cacerts []byte, hash string) (bool, string, string) {
	if len(cacerts) == 0 && hash == "" {
		return true, "", ""
	}

	digest := sha256.Sum256([]byte(cacerts))
	newHash := hex.EncodeToString(digest[:])
	return hash == newHash, hash, newHash
}

func ParseUsernamePassword(token string) (string, string, bool) {
	parsed, err := parseToken(token)
	if err != nil {
		return "", "", false
	}
	return parsed.username, parsed.password, true
}

func parseToken(token string) (clientToken, error) {
	var result clientToken

	if !strings.HasPrefix(token, "K10") {
		return result, fmt.Errorf("token is not a valid token format")
	}

	token = token[3:]

	parts := strings.SplitN(token, "::", 2)
	token = parts[0]
	if len(parts) > 1 {
		result.caHash = parts[0]
		token = parts[1]
	}

	parts = strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return result, fmt.Errorf("token credentials are the wrong format")
	}

	result.username = parts[0]
	result.password = parts[1]

	return result, nil
}

func GetHTTPClient(cacerts []byte) *http.Client {
	if len(cacerts) == 0 {
		return http.DefaultClient
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(cacerts)

	return &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
	}
}

func Get(path string, info *Info) ([]byte, error) {
	u, err := url.Parse(info.URL)
	if err != nil {
		return nil, err
	}
	u.Path = path
	return get(u.String(), GetHTTPClient(info.CACerts), info.username, info.password)
}

func GetCACerts(u url.URL) ([]byte, error) {
	u.Path = "/cacerts"
	url := u.String()

	_, err := get(url, http.DefaultClient, "", "")
	if err == nil {
		return nil, nil
	}

	cacerts, err := get(url, insecureClient, "", "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get CA certs at %s", url)
	}

	_, err = get(url, GetHTTPClient(cacerts), "", "")
	if err != nil {
		return nil, errors.Wrapf(err, "server %s is not trusted", url)
	}

	return cacerts, nil
}

func get(u string, client *http.Client, username, password string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	if username != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", u, resp.Status)
	}

	return ioutil.ReadAll(resp.Body)
}
