package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/k3s/pkg/daemons/config"
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

func doAuth(roles []string, serverConfig *config.Control, next http.Handler, rw http.ResponseWriter, req *http.Request) {
	switch {
	case serverConfig == nil:
		logrus.Errorf("Authenticate not initialized: serverConfig is nil")
		rw.WriteHeader(http.StatusUnauthorized)
		return
	case serverConfig.Runtime.Authenticator == nil:
		logrus.Errorf("Authenticate not initialized: serverConfig.Runtime.Authenticator is nil")
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	resp, ok, err := serverConfig.Runtime.Authenticator.AuthenticateRequest(req)
	if err != nil {
		logrus.Errorf("Failed to authenticate request from %s: %v", req.RemoteAddr, err)
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !ok || !hasRole(roles, resp.User.GetGroups()) {
		rw.WriteHeader(http.StatusForbidden)
		return
	}

	ctx := request.WithUser(req.Context(), resp.User)
	req = req.WithContext(ctx)
	next.ServeHTTP(rw, req)
}

func authMiddleware(serverConfig *config.Control, roles ...string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			doAuth(roles, serverConfig, next, rw, req)
		})
	}
}
