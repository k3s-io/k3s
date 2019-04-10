package server

import (
	"crypto/rsa"
	"crypto/x509"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/openapi"
	certutil "github.com/rancher/norman/pkg/cert"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/kubernetes/pkg/master"
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
	router.Path("/client-cacerts").Handler(clientcacerts(serverConfig))
	router.Path("/server-cacerts").Handler(servercacerts(serverConfig))
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

		if req.Method != http.MethodPost {
			resp.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		logrus.Info(req.Header)
		var nodeName string
		nodeNames := req.Header["K3s-Node-Name"]
		if len(nodeNames) == 1 {
			nodeName = nodeNames[0]
		} else {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}

		nodeKey, err := ioutil.ReadAll(req.Body)
		if err != nil {
			sendError(err, resp)
			return
		}

		key, err := certutil.ParsePrivateKeyPEM(nodeKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		caKeyBytes, err := ioutil.ReadFile(server.Runtime.ServerCAKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		caBytes, err := ioutil.ReadFile(server.Runtime.ServerCA)
		if err != nil {
			sendError(err, resp)
			return
		}

		caKey, err := certutil.ParsePrivateKeyPEM(caKeyBytes)
		if err != nil {
			sendError(err, resp)
			return
		}

		caCert, err := certutil.ParseCertsPEM(caBytes)
		if err != nil {
			sendError(err, resp)
			return
		}

		_, apiServerServiceIP, err := master.DefaultServiceIPRange(*server.ServiceIPRange)
		if err != nil {
			sendError(err, resp)
			return
		}

		cfg := certutil.Config{
			CommonName:   "system:node:" + nodeName,
			Organization: []string{"system:nodes"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			AltNames: certutil.AltNames{
				DNSNames: []string{"kubernetes.default.svc", "kubernetes.default", "kubernetes", "localhost"},
				IPs:      []net.IP{apiServerServiceIP, net.ParseIP("127.0.0.1")},
			},
		}

		cert, err := certutil.NewSignedCert(cfg, key.(*rsa.PrivateKey), caCert[0], caKey.(*rsa.PrivateKey))
		if err != nil {
			sendError(err, resp)
			return
		}

		// serverCABytes, err := ioutil.ReadFile(server.Runtime.ServerCA)
		// if err != nil {
		// 	sendError(err, resp)
		// 	return
		// }

		// serverCACert, err := certutil.ParseCertsPEM(serverCABytes)
		// if err != nil {
		// 	sendError(err, resp)
		// 	return
		// }

		// certs := append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(servingCACert[0])...)

		certs := certutil.EncodeCertPEM(cert)
		// certs = append(certs, certutil.EncodeCertPEM(caCert[0])...)
		// certs = append(certs, certutil.EncodeCertPEM(serverCACert[0])...)
		resp.Write(certs)
		// http.ServeFile(resp, req, server.Runtime.ServingKubeAPICert)
	})
}

func nodeKey(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(resp, req, server.Runtime.ServingKubeAPIKey)
	})
}

func clientcacerts(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(resp, req, server.Runtime.ClientCA)
	})
}

func servercacerts(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(resp, req, server.Runtime.ServerCA)
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
			sendError(err, resp)
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

func sendError(err error, resp http.ResponseWriter) {
	logrus.Error(err)
	resp.WriteHeader(http.StatusInternalServerError)
	resp.Write([]byte(err.Error()))
}
