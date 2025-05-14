package auth

import (
	"errors"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/apis/apiserver"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/kubernetes/scheme"
)

var (
	requestInfoResolver = &apirequest.RequestInfoFactory{}
	failedHandler       = genericapifilters.Unauthorized(scheme.Codecs)
)

func hasRole(mustRoles []string, roles []string) bool {
	for _, check := range roles {
		for _, role := range mustRoles {
			if role == check {
				return true
			}
		}
	}
	return false
}

// doAuth calls the cluster's authenticator to validate that the client has at least one of the listed roles
func doAuth(roles []string, serverConfig *config.Control, next http.Handler, rw http.ResponseWriter, req *http.Request) {
	switch {
	case serverConfig == nil:
		logrus.Errorf("Authenticate not initialized: serverConfig is nil")
		util.SendError(errors.New("not authorized"), rw, req, http.StatusUnauthorized)
		return
	case serverConfig.Runtime.Authenticator == nil:
		logrus.Errorf("Authenticate not initialized: serverConfig.Runtime.Authenticator is nil")
		util.SendError(errors.New("not authorized"), rw, req, http.StatusUnauthorized)
		return
	}

	resp, ok, err := serverConfig.Runtime.Authenticator.AuthenticateRequest(req)
	if err != nil {
		logrus.Errorf("Failed to authenticate request from %s: %v", req.RemoteAddr, err)
		util.SendError(errors.New("not authorized"), rw, req, http.StatusUnauthorized)
		return
	}

	if !ok || !hasRole(roles, resp.User.GetGroups()) {
		util.SendError(errors.New("forbidden"), rw, req, http.StatusForbidden)
		return
	}

	ctx := apirequest.WithUser(req.Context(), resp.User)
	req = req.WithContext(ctx)
	next.ServeHTTP(rw, req)
}

// HasRole returns a middleware function that validates that the request
// is being made with at least one of the listed roles.
func HasRole(serverConfig *config.Control, roles ...string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			doAuth(roles, serverConfig, next, rw, req)
		})
	}
}

// IsLocalOrHasRole returns a middleware function that validates that the request
// is from a local client or has at least one of the listed roles.
func IsLocalOrHasRole(serverConfig *config.Control, roles ...string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			client, _, _ := net.SplitHostPort(req.RemoteAddr)
			if client == "127.0.0.1" || client == "::1" {
				next.ServeHTTP(rw, req)
			} else {
				doAuth(roles, serverConfig, next, rw, req)
			}
		})
	}
}

// Delegated returns a middleware function that uses core Kubernetes
// authentication/authorization via client certificate auth and the SubjectAccessReview API
func Delegated(clientCA, kubeConfig string, config *server.Config) mux.MiddlewareFunc {
	if config == nil {
		config = &server.Config{}
	}

	authn := options.NewDelegatingAuthenticationOptions()
	authn.Anonymous = &apiserver.AnonymousAuthConfig{
		Enabled: false,
	}
	authn.SkipInClusterLookup = true
	authn.ClientCert = options.ClientCertAuthenticationOptions{
		ClientCA: clientCA,
	}
	authn.RemoteKubeConfigFile = kubeConfig
	if err := authn.ApplyTo(&config.Authentication, config.SecureServing, nil); err != nil {
		logrus.Fatalf("Failed to apply authentication configuration: %v", err)
	}

	authz := options.NewDelegatingAuthorizationOptions()
	authz.AlwaysAllowPaths = []string{ // skip authz for paths that should not use SubjectAccessReview; basically everything that will use this router other than metrics
		"/v1-" + version.Program + "/p2p", // spegel libp2p peer discovery
		"/v2/*",                           // spegel registry mirror
		"/debug/pprof/*",                  // profiling
	}
	authz.RemoteKubeConfigFile = kubeConfig
	if err := authz.ApplyTo(&config.Authorization); err != nil {
		logrus.Fatalf("Failed to apply authorization configuration: %v", err)
	}

	return func(handler http.Handler) http.Handler {
		handler = genericapifilters.WithAuthorization(handler, config.Authorization.Authorizer, scheme.Codecs)
		handler = genericapifilters.WithAuthentication(handler, config.Authentication.Authenticator, failedHandler, nil, nil)
		handler = genericapifilters.WithCacheControl(handler)
		return handler
	}
}

// RequestInfo returns a middleware function that adds verb/resource/gvk/etc info to the request context.
// This must be set for other filters to function, but only needs to be in each middleware chain once.
func RequestInfo() mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		return genericapifilters.WithRequestInfo(handler, requestInfoResolver)
	}
}

// MaxInFlight returns a middleware function that limits the number of requests that are executed concurrently.
// This is not strictly auth related, but it also uses the core Kubernetes request filters.
func MaxInFlight(nonMutatingLimit, mutatingLimit int) mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		return genericfilters.WithMaxInFlightLimit(handler, nonMutatingLimit, mutatingLimit, nil)
	}
}
