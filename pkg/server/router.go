package server

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/openapi"
	"k8s.io/apimachinery/pkg/util/json"
)

const (
	jsonMediaType   = "application/json"
	binaryMediaType = "application/octet-stream"
	pbMediaType     = "application/com.github.proto-openapi.spec.v2@v1.0+protobuf"
	openapiPrefix   = "openapi."
	staticURL       = "/static/"
)

type CACertsGetter func() (string, error)

func router(serverConfig *config.Control, tunnel http.Handler, cacertsGetter CACertsGetter) http.Handler {
	authed := mux.NewRouter()
	authed.Use(authMiddleware(serverConfig))
	authed.NotFoundHandler = serverConfig.Runtime.Handler
	authed.Path("/v1-k3s/connect").Handler(tunnel)
	authed.Path("/v1-k3s/node.crt").Handler(nodeCrt(serverConfig))
	authed.Path("/v1-k3s/node.key").Handler(nodeKey(serverConfig))
	authed.Path("/v1-k3s/config").Handler(configHandler(serverConfig))

	staticDir := filepath.Join(serverConfig.DataDir, "static")
	router := mux.NewRouter()
	router.NotFoundHandler = authed
	router.PathPrefix(staticURL).Handler(serveStatic(staticURL, staticDir))
	router.Path("/cacerts").Handler(cacerts(cacertsGetter))
	router.Path("/openapi/v2").Handler(serveOpenapi())
	router.Path("/ping").Handler(ping())

	return router
}

func cacerts(getter CACertsGetter) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		content, err := getter()
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			resp.Write([]byte(err.Error()))
		}
		resp.Header().Set("content-type", "text/plain")
		resp.Write([]byte(content))
	})
}

func nodeCrt(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(resp, req, server.Runtime.NodeCert)
	})
}

func nodeKey(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(resp, req, server.Runtime.NodeKey)
	})
}

func configHandler(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		resp.Header().Set("content-type", "application/json")
		json.NewEncoder(resp).Encode(server)
	})
}

func serveOpenapi() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		suffix := "json"
		contentType := jsonMediaType
		if req.Header.Get("Accept") == pbMediaType {
			suffix = "pb"
			contentType = binaryMediaType
		}

		data, err := openapi.Asset(openapiPrefix + suffix)
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			resp.Write([]byte(err.Error()))
			return
		}

		resp.Header().Set("Content-Type", contentType)
		resp.Header().Set("Content-Length", strconv.Itoa(len(data)))
		resp.Write(data)
	})
}

func ping() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		data := []byte("pong")
		resp.Header().Set("Content-Type", "text/plain")
		resp.Header().Set("Content-Length", strconv.Itoa(len(data)))
		resp.Write(data)
	})
}

func serveStatic(urlPrefix, staticDir string) http.Handler {
	return http.StripPrefix(urlPrefix, http.FileServer(http.Dir(staticDir)))
}
