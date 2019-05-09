package server

import (
	"bufio"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/openapi"
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

		nodeNames := req.Header["K3s-Node-Name"]
		if len(nodeNames) != 1 || nodeNames[0] == "" {
			sendError(errors.New("node name not set"), resp)
			return
		}

		nodePasswords := req.Header["K3s-Node-Password"]
		if len(nodePasswords) != 1 || nodePasswords[0] == "" {
			sendError(errors.New("node password not set"), resp)
			return
		}

		if err := ensureNodePassword(server.Runtime.PasswdFile, nodeNames[0], nodePasswords[0]); err != nil {
			sendError(err, resp, http.StatusForbidden)
			return
		}

		nodeKey, err := ioutil.ReadFile(server.Runtime.NodeKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		key, err := certutil.ParsePrivateKeyPEM(nodeKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		caKeyBytes, err := ioutil.ReadFile(server.Runtime.TokenCAKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		caBytes, err := ioutil.ReadFile(server.Runtime.TokenCA)
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
			CommonName: "kubernetes",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			AltNames: certutil.AltNames{
				DNSNames: []string{"kubernetes.default.svc", "kubernetes.default", "kubernetes", "localhost", nodeNames[0]},
				IPs:      []net.IP{apiServerServiceIP, net.ParseIP("127.0.0.1")},
			},
		}

		cert, err := certutil.NewSignedCert(cfg, key.(*rsa.PrivateKey), caCert[0], caKey.(*rsa.PrivateKey))
		if err != nil {
			sendError(err, resp)
			return
		}

		resp.Write(append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(caCert[0])...))
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

func sendError(err error, resp http.ResponseWriter, status ...int) {
	code := http.StatusInternalServerError
	if len(status) == 1 {
		code = status[0]
	}

	logrus.Error(err)
	resp.WriteHeader(code)
	resp.Write([]byte(err.Error()))
}

func ensureNodePassword(passwdFile, nodeName, passwd string) error {
	f, err := os.Open(passwdFile)
	if err != nil {
		return err
	}
	defer f.Close()
	user := strings.ToLower("node:" + nodeName)

	buf := &strings.Builder{}
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := scan.Text()
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		if parts[1] == user {
			if parts[0] == passwd {
				return nil
			}
			return fmt.Errorf("Node password validation failed for [%s]", nodeName)
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	buf.WriteString(fmt.Sprintf("%s,%s,%s,system:masters\n", passwd, user, user))

	if scan.Err() != nil {
		return scan.Err()
	}

	f.Close()
	return ioutil.WriteFile(passwdFile, []byte(buf.String()), 0600)
}
