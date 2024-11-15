package loadbalancer

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	http_dialer "github.com/mwitkow/go-http-dialer"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/proxy"
)

var defaultDialer proxy.Dialer = &net.Dialer{
	Timeout:   10 * time.Second,
	KeepAlive: 30 * time.Second,
}

// SetHTTPProxy configures a proxy-enabled dialer to be used for all loadbalancer connections,
// if the agent has been configured to allow use of a HTTP proxy, and the environment has been configured
// to indicate use of a HTTP proxy for the server URL.
func SetHTTPProxy(address string) error {
	// Check if env variable for proxy is set
	if useProxy, _ := strconv.ParseBool(os.Getenv(version.ProgramUpper + "_AGENT_HTTP_PROXY_ALLOWED")); !useProxy || address == "" {
		return nil
	}

	serverURL, err := url.Parse(address)
	if err != nil {
		return errors.Wrapf(err, "failed to parse address %s", address)
	}

	// Call this directly instead of using the cached environment used by http.ProxyFromEnvironment to allow for testing
	proxyFromEnvironment := httpproxy.FromEnvironment().ProxyFunc()
	proxyURL, err := proxyFromEnvironment(serverURL)
	if err != nil {
		return errors.Wrapf(err, "failed to get proxy for address %s", address)
	}
	if proxyURL == nil {
		logrus.Debug(version.ProgramUpper + "_AGENT_HTTP_PROXY_ALLOWED is true but no proxy is configured for URL " + serverURL.String())
		return nil
	}

	dialer, err := proxyDialer(proxyURL, defaultDialer)
	if err != nil {
		return errors.Wrapf(err, "failed to create proxy dialer for %s", proxyURL)
	}

	defaultDialer = dialer
	logrus.Debugf("Using proxy %s for agent connection to %s", proxyURL, serverURL)
	return nil
}

// proxyDialer creates a new proxy.Dialer that routes connections through the specified proxy.
func proxyDialer(proxyURL *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	if proxyURL.Scheme == "http" || proxyURL.Scheme == "https" {
		// Create a new HTTP proxy dialer
		httpProxyDialer := http_dialer.New(proxyURL, http_dialer.WithConnectionTimeout(10*time.Second), http_dialer.WithDialer(forward.(*net.Dialer)))
		return httpProxyDialer, nil
	} else if proxyURL.Scheme == "socks5" {
		// For SOCKS5 proxies, use the proxy package's FromURL
		return proxy.FromURL(proxyURL, forward)
	}
	return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
}
