package server

import (
	"crypto"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/nodepassword"
	"github.com/rancher/k3s/pkg/version"
	coreclient "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/json"
)

const (
	staticURL = "/static/"
)

func router(serverConfig *config.Control) http.Handler {
	prefix := "/v1-" + version.Program
	authed := mux.NewRouter()
	authed.Use(authMiddleware(serverConfig, version.Program+":agent"))
	authed.NotFoundHandler = serverConfig.Runtime.Handler
	authed.Path(prefix + "/serving-kubelet.crt").Handler(servingKubeletCert(serverConfig, serverConfig.Runtime.ServingKubeletKey, serverConfig.Runtime))
	authed.Path(prefix + "/client-kubelet.crt").Handler(clientKubeletCert(serverConfig, serverConfig.Runtime.ClientKubeletKey, serverConfig.Runtime))
	authed.Path(prefix + "/client-kube-proxy.crt").Handler(fileHandler(serverConfig.Runtime.ClientKubeProxyCert, serverConfig.Runtime.ClientKubeProxyKey))
	authed.Path(prefix + "/client-" + version.Program + "-controller.crt").Handler(fileHandler(serverConfig.Runtime.ClientK3sControllerCert, serverConfig.Runtime.ClientK3sControllerKey))
	authed.Path(prefix + "/client-ca.crt").Handler(fileHandler(serverConfig.Runtime.ClientCA))
	authed.Path(prefix + "/server-ca.crt").Handler(fileHandler(serverConfig.Runtime.ServerCA))
	authed.Path(prefix + "/config").Handler(configHandler(serverConfig))

	nodeAuthed := mux.NewRouter()
	nodeAuthed.Use(authMiddleware(serverConfig, "system:nodes"))
	nodeAuthed.Path(prefix + "/connect").Handler(serverConfig.Runtime.Tunnel)
	nodeAuthed.NotFoundHandler = authed

	serverAuthed := mux.NewRouter()
	serverAuthed.Use(authMiddleware(serverConfig, version.Program+":server"))
	serverAuthed.NotFoundHandler = nodeAuthed
	serverAuthed.Path("/db/info").Handler(nodeAuthed)
	if serverConfig.Runtime.HTTPBootstrap {
		serverAuthed.Path(prefix + "/server-bootstrap").Handler(bootstrap.Handler(&serverConfig.Runtime.ControlRuntimeBootstrap))
	}

	staticDir := filepath.Join(serverConfig.DataDir, "static")
	router := mux.NewRouter()
	router.NotFoundHandler = serverAuthed
	router.PathPrefix(staticURL).Handler(serveStatic(staticURL, staticDir))
	router.Path("/cacerts").Handler(cacerts(serverConfig.Runtime.ServerCA))
	router.Path("/ping").Handler(ping())

	return router
}

func cacerts(serverCA string) http.Handler {
	var ca []byte
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if ca == nil {
			var err error
			ca, err = ioutil.ReadFile(serverCA)
			if err != nil {
				sendError(err, resp)
				return
			}
		}
		resp.Header().Set("content-type", "text/plain")
		resp.Write(ca)
	})
}

func getNodeInfo(req *http.Request) (string, string, error) {
	nodeName := req.Header.Get(version.Program + "-Node-Name")
	if nodeName == "" {
		return "", "", errors.New("node name not set")
	}

	nodePassword := req.Header.Get(version.Program + "-Node-Password")
	if nodePassword == "" {
		return "", "", errors.New("node password not set")
	}

	return strings.ToLower(nodeName), nodePassword, nil
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

func servingKubeletCert(server *config.Control, keyFile string, runtime *config.ControlRuntime) http.Handler {
	var secretClient coreclient.SecretClient
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if secretClient == nil {
			if runtime.Core == nil {
				sendError(errors.New("runtime core not ready"), resp)
				return
			}
			secretClient = runtime.Core.Core().V1().Secret()
		}

		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		nodeName, nodePassword, err := getNodeInfo(req)
		if err != nil {
			sendError(err, resp)
			return
		}

		if err := nodepassword.Ensure(secretClient, nodeName, nodePassword); err != nil {
			sendError(err, resp, http.StatusForbidden)
			return
		}

		caCert, caKey, key, err := getCACertAndKeys(server.Runtime.ServerCA, server.Runtime.ServerCAKey, server.Runtime.ServingKubeletKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		ips := []net.IP{net.ParseIP("127.0.0.1")}
		if nodeIP := req.Header.Get(version.Program + "-Node-IP"); nodeIP != "" {
			ips = append(ips, net.ParseIP(nodeIP))
		}

		cert, err := certutil.NewSignedCert(certutil.Config{
			CommonName: nodeName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			AltNames: certutil.AltNames{
				DNSNames: []string{nodeName, "localhost"},
				IPs:      ips,
			},
		}, key, caCert[0], caKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		keyBytes, err := ioutil.ReadFile(keyFile)
		if err != nil {
			http.Error(resp, err.Error(), http.StatusInternalServerError)
			return
		}

		resp.Write(append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(caCert[0])...))
		resp.Write(keyBytes)
	})
}

func clientKubeletCert(server *config.Control, keyFile string, runtime *config.ControlRuntime) http.Handler {
	var secretClient coreclient.SecretClient
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if secretClient == nil {
			if runtime.Core == nil {
				sendError(errors.New("runtime core not ready"), resp)
				return
			}
			secretClient = runtime.Core.Core().V1().Secret()
		}

		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		nodeName, nodePassword, err := getNodeInfo(req)
		if err != nil {
			sendError(err, resp)
			return
		}

		if err := nodepassword.Ensure(secretClient, nodeName, nodePassword); err != nil {
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

		keyBytes, err := ioutil.ReadFile(keyFile)
		if err != nil {
			http.Error(resp, err.Error(), http.StatusInternalServerError)
			return
		}

		resp.Write(append(certutil.EncodeCertPEM(cert), certutil.EncodeCertPEM(caCert[0])...))
		resp.Write(keyBytes)
	})
}

func fileHandler(fileName ...string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		resp.Header().Set("Content-Type", "text/plain")

		if len(fileName) == 1 {
			http.ServeFile(resp, req, fileName[0])
			return
		}

		for _, f := range fileName {
			bytes, err := ioutil.ReadFile(f)
			if err != nil {
				logrus.Errorf("Failed to read %s: %v", f, err)
				resp.WriteHeader(http.StatusInternalServerError)
				return
			}
			resp.Write(bytes)
		}
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
