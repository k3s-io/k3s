package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/endpoints/request"
)

func doAuth(serverConfig *config.Control, next http.Handler, rw http.ResponseWriter, req *http.Request) {
	if serverConfig == nil || serverConfig.Runtime.Authenticator == nil {
		next.ServeHTTP(rw, req)
		return
	}

	resp, ok, err := serverConfig.Runtime.Authenticator.AuthenticateRequest(req)
	if err != nil {
		logrus.Errorf("failed to authenticate request: %v", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !ok || resp.User.GetName() != "node" {
		rw.WriteHeader(http.StatusUnauthorized)
		return
	}

	ctx := request.WithUser(req.Context(), resp.User)
	req = req.WithContext(ctx)
	next.ServeHTTP(rw, req)
}

func authMiddleware(serverConfig *config.Control) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			doAuth(serverConfig, next, rw, req)
		})
	}
}
