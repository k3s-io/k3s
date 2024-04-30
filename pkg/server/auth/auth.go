package auth

import (
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/endpoints/request"
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

	ctx := request.WithUser(req.Context(), resp.User)
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
