package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
)

var (
	er = &errorResponder{}
)

type errorResponder struct {
}

func (e *errorResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

type SimpleProxy struct {
	url                *url.URL
	transport          http.RoundTripper
	overrideHostHeader bool
}

func NewSimpleProxy(host string, caData []byte, overrideHostHeader bool) (*SimpleProxy, error) {
	hostURL, _, err := rest.DefaultServerURL(host, "", schema.GroupVersion{}, true)
	if err != nil {
		return nil, err
	}

	ht := &http.Transport{}
	if len(caData) > 0 {
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caData)
		ht.TLSClientConfig = &tls.Config{
			RootCAs: certPool,
		}
	}

	return &SimpleProxy{
		url:                hostURL,
		transport:          ht,
		overrideHostHeader: overrideHostHeader,
	}, nil
}

func (s *SimpleProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	u := *s.url
	u.Path = req.URL.Path
	u.RawQuery = req.URL.RawQuery
	req.URL.Scheme = "https"
	req.URL.Host = req.Host
	if s.overrideHostHeader {
		req.Host = u.Host
	}
	httpProxy := proxy.NewUpgradeAwareHandler(&u, s.transport, false, false, er)
	httpProxy.ServeHTTP(rw, req)
}
