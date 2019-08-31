package server

import (
	"crypto"
	"crypto/x509"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
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
	"github.com/rancher/k3s/pkg/daemons/control"
	"github.com/sirupsen/logrus"
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
	authed.Path("/v1-k3s/serving-kubelet.crt").Handler(servingKubeletCert(serverConfig))
	authed.Path("/v1-k3s/serving-kubelet.key").Handler(fileHandler(serverConfig.Runtime.ServingKubeletKey))
	authed.Path("/v1-k3s/client-kubelet.crt").Handler(clientKubeletCert(serverConfig))
	authed.Path("/v1-k3s/client-kubelet.key").Handler(fileHandler(serverConfig.Runtime.ClientKubeletKey))
	authed.Path("/v1-k3s/client-kube-proxy.crt").Handler(fileHandler(serverConfig.Runtime.ClientKubeProxyCert))
	authed.Path("/v1-k3s/client-kube-proxy.key").Handler(fileHandler(serverConfig.Runtime.ClientKubeProxyKey))
	authed.Path("/v1-k3s/client-ca.crt").Handler(fileHandler(serverConfig.Runtime.ClientCA))
	authed.Path("/v1-k3s/server-ca.crt").Handler(fileHandler(serverConfig.Runtime.ServerCA))
	authed.Path("/v1-k3s/config").Handler(configHandler(serverConfig))

	staticDir := filepath.Join(serverConfig.DataDir, "static")
	router := mux.NewRouter()
	router.NotFoundHandler = authed
	router.PathPrefix(staticURL).Handler(serveStatic(staticURL, staticDir))
	router.Path("/cacerts").Handler(cacerts(cacertsGetter))
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

func getNodeInfo(req *http.Request) (string, string, error) {
	nodeNames := req.Header["K3s-Node-Name"]
	if len(nodeNames) != 1 || nodeNames[0] == "" {
		return "", "", errors.New("node name not set")
	}

	nodePasswords := req.Header["K3s-Node-Password"]
	if len(nodePasswords) != 1 || nodePasswords[0] == "" {
		return "", "", errors.New("node password not set")
	}

	return strings.ToLower(nodeNames[0]), nodePasswords[0], nil
}

func getCACertAndKeys(caCertFile, caKeyFile, signingKeyFile string) ([]*x509.Certificate, crypto.Signer, crypto.Signer, error) {
	keyBytes, err := ioutil.ReadFile(signingKeyFile)
	if err != nil {
		return nil, nil, nil, err
	}

	key, err := certutil.ParsePrivateKeyPEM(keyBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	caKeyBytes, err := ioutil.ReadFile(caKeyFile)
	if err != nil {
		return nil, nil, nil, err
	}

	caKey, err := certutil.ParsePrivateKeyPEM(caKeyBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	caBytes, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		return nil, nil, nil, err
	}

	caCert, err := certutil.ParseCertsPEM(caBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	return caCert, caKey.(crypto.Signer), key.(crypto.Signer), nil
}

func servingKubeletCert(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		nodeName, nodePassword, err := getNodeInfo(req)
		if err != nil {
			sendError(err, resp)
			return
		}

		if err := ensureNodePassword(server.Runtime.NodePasswdFile, nodeName, nodePassword); err != nil {
			sendError(err, resp, http.StatusForbidden)
			return
		}

		caCert, caKey, key, err := getCACertAndKeys(server.Runtime.ServerCA, server.Runtime.ServerCAKey, server.Runtime.ServingKubeletKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		cert, err := certutil.NewSignedCert(certutil.Config{
			CommonName: nodeName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			AltNames: certutil.AltNames{
				DNSNames: []string{nodeName, "localhost"},
				IPs:      []net.IP{net.ParseIP("127.0.0.1")},
			},
		}, key, caCert[0], caKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		resp.Write(append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(caCert[0])...))
	})
}

func clientKubeletCert(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		nodeName, nodePassword, err := getNodeInfo(req)
		if err != nil {
			sendError(err, resp)
			return
		}

		if err := ensureNodePassword(server.Runtime.NodePasswdFile, nodeName, nodePassword); err != nil {
			sendError(err, resp, http.StatusForbidden)
			return
		}

		caCert, caKey, key, err := getCACertAndKeys(server.Runtime.ClientCA, server.Runtime.ClientCAKey, server.Runtime.ClientKubeletKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		cert, err := certutil.NewSignedCert(certutil.Config{
			CommonName:   "system:node:" + nodeName,
			Organization: []string{"system:nodes"},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}, key, caCert[0], caKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		resp.Write(append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(caCert[0])...))
	})
}

func fileHandler(fileName string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(resp, req, fileName)
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
	records := [][]string{}

	if _, err := os.Stat(passwdFile); !os.IsNotExist(err) {
		f, err := os.Open(passwdFile)
		if err != nil {
			return err
		}
		defer f.Close()
		reader := csv.NewReader(f)
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if len(record) < 2 {
				return fmt.Errorf("password file '%s' must have at least 2 columns (password, nodeName), found %d", passwdFile, len(record))
			}
			if record[1] == nodeName {
				if record[0] == passwd {
					return nil
				}
				return fmt.Errorf("Node password validation failed for '%s', using passwd file '%s'", nodeName, passwdFile)
			}
			records = append(records, record)
		}
		f.Close()
	}

	records = append(records, []string{passwd, nodeName})
	return control.WritePasswords(passwdFile, records)
}
