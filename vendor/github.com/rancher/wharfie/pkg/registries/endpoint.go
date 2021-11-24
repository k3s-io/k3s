package registries

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sirupsen/logrus"
)

var _ authn.Keychain = &endpoint{}
var _ http.RoundTripper = &endpoint{}

type endpoint struct {
	auth     authn.Authenticator
	keychain authn.Keychain
	ref      name.Reference
	registry *registry
	url      *url.URL
}

// Resolve returns an authenticator for the authn.Keychain interface. The authenticator
// provides credentials to a registry by returning the credentials from mirror endpoints.
// If there were no credentials provided for this endpoint, the default keychain is used
// as a fallback, followed by simply anonymous access.
func (e endpoint) Resolve(target authn.Resource) (authn.Authenticator, error) {
	if e.auth != nil && e.auth != authn.Anonymous {
		return e.auth, nil
	}
	if e.keychain != nil {
		return e.keychain.Resolve(target)
	}
	return authn.Anonymous, nil
}

// RoundTrip handles making a request to an endpoint. It is responsible for rewriting the request
// URL to reflect the scheme, host, and path specified in the endpoint config. The transport itself
// will be retrieved from the registry config, potentially using a cached entry.
func (e endpoint) RoundTrip(req *http.Request) (*http.Response, error) {
	endpointURL := e.url
	originalURL := req.URL.String()

	// Only rewrite the URL if the request is being made against the original registry host
	// and endpoint.  We might have been redirected to a different URL as part of the auth
	// workflow, and must not rewrite URLs if that's the case.
	if req.URL.Host == e.ref.Context().RegistryStr() && strings.HasPrefix(req.URL.Path, "/v2") {
		// The default base path is /v2/; if a path is included in the endpoint,
		// replace the /v2/ prefix from the request path with the endpoint path.
		// This behavior is cribbed from containerd.
		if endpointURL.Path != "" {
			req.URL.Path = endpointURL.Path + strings.TrimPrefix(req.URL.Path, "/v2")

			// If either URL has RawPath set (due to the path including urlencoded
			// characters), it also needs to be used to set the combined URL
			if endpointURL.RawPath != "" || req.URL.RawPath != "" {
				endpointPath := endpointURL.Path
				if endpointURL.RawPath != "" {
					endpointPath = endpointURL.RawPath
				}
				reqPath := req.URL.Path
				if req.URL.RawPath != "" {
					reqPath = req.URL.RawPath
				}
				req.URL.RawPath = endpointPath + strings.TrimPrefix(reqPath, "/v2")
			}
		}

		// override request host and scheme
		req.Host = endpointURL.Host
		req.URL.Host = endpointURL.Host
		req.URL.Scheme = endpointURL.Scheme
	}

	if newURL := req.URL.String(); originalURL != newURL {
		logrus.Debugf("Registry endpoint URL modified: %s => %s", originalURL, newURL)
	}
	return e.registry.getTransport(req.URL).RoundTrip(req)
}
