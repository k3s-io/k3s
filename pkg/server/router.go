package server

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/generated/clientset/versioned/scheme"
	"github.com/k3s-io/k3s/pkg/nodepassword"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	coreclient "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
)

const (
	staticURL = "/static/"
)

func router(ctx context.Context, config *Config, cfg *cmds.Server) http.Handler {
	serverConfig := &config.ControlConfig
	nodeAuth := passwordBootstrap(ctx, config)

	prefix := "/v1-" + version.Program
	authed := mux.NewRouter().SkipClean(true)
	authed.Use(authMiddleware(serverConfig, version.Program+":agent"))
	authed.Path(prefix + "/serving-kubelet.crt").Handler(servingKubeletCert(serverConfig, serverConfig.Runtime.ServingKubeletKey, nodeAuth))
	authed.Path(prefix + "/client-kubelet.crt").Handler(clientKubeletCert(serverConfig, serverConfig.Runtime.ClientKubeletKey, nodeAuth))
	authed.Path(prefix + "/client-kube-proxy.crt").Handler(fileHandler(serverConfig.Runtime.ClientKubeProxyCert, serverConfig.Runtime.ClientKubeProxyKey))
	authed.Path(prefix + "/client-" + version.Program + "-controller.crt").Handler(fileHandler(serverConfig.Runtime.ClientK3sControllerCert, serverConfig.Runtime.ClientK3sControllerKey))
	authed.Path(prefix + "/client-ca.crt").Handler(fileHandler(serverConfig.Runtime.ClientCA))
	authed.Path(prefix + "/server-ca.crt").Handler(fileHandler(serverConfig.Runtime.ServerCA))
	authed.Path(prefix + "/apiservers").Handler(apiserversHandler(serverConfig))
	authed.Path(prefix + "/config").Handler(configHandler(serverConfig, cfg))
	authed.Path(prefix + "/readyz").Handler(readyzHandler(serverConfig))

	if cfg.DisableAPIServer {
		authed.NotFoundHandler = apiserverDisabled()
	} else {
		authed.NotFoundHandler = apiserver(serverConfig.Runtime)
	}

	nodeAuthed := mux.NewRouter().SkipClean(true)
	nodeAuthed.NotFoundHandler = authed
	nodeAuthed.Use(authMiddleware(serverConfig, user.NodesGroup))
	nodeAuthed.Path(prefix + "/connect").Handler(serverConfig.Runtime.Tunnel)

	serverAuthed := mux.NewRouter().SkipClean(true)
	serverAuthed.NotFoundHandler = nodeAuthed
	serverAuthed.Use(authMiddleware(serverConfig, version.Program+":server"))
	serverAuthed.Path(prefix + "/encrypt/status").Handler(encryptionStatusHandler(serverConfig))
	serverAuthed.Path(prefix + "/encrypt/config").Handler(encryptionConfigHandler(ctx, serverConfig))
	serverAuthed.Path("/db/info").Handler(nodeAuthed)
	serverAuthed.Path(prefix + "/server-bootstrap").Handler(bootstrapHandler(serverConfig.Runtime))

	systemAuthed := mux.NewRouter().SkipClean(true)
	systemAuthed.NotFoundHandler = serverAuthed
	systemAuthed.MethodNotAllowedHandler = serverAuthed
	systemAuthed.Use(authMiddleware(serverConfig, user.SystemPrivilegedGroup))
	systemAuthed.Methods(http.MethodConnect).Handler(serverConfig.Runtime.Tunnel)

	staticDir := filepath.Join(serverConfig.DataDir, "static")
	router := mux.NewRouter().SkipClean(true)
	router.NotFoundHandler = systemAuthed
	router.PathPrefix(staticURL).Handler(serveStatic(staticURL, staticDir))
	router.Path("/cacerts").Handler(cacerts(serverConfig.Runtime.ServerCA))
	router.Path("/ping").Handler(ping())

	return router
}

func apiserver(runtime *config.ControlRuntime) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if runtime != nil && runtime.APIServer != nil {
			runtime.APIServer.ServeHTTP(resp, req)
		} else {
			responsewriters.ErrorNegotiated(
				apierrors.NewServiceUnavailable("apiserver not ready"),
				scheme.Codecs.WithoutConversion(), schema.GroupVersion{}, resp, req,
			)
		}
	})
}

func apiserverDisabled() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		responsewriters.ErrorNegotiated(
			apierrors.NewServiceUnavailable("apiserver disabled"),
			scheme.Codecs.WithoutConversion(), schema.GroupVersion{}, resp, req,
		)
	})
}

func bootstrapHandler(runtime *config.ControlRuntime) http.Handler {
	if runtime.HTTPBootstrap {
		return bootstrap.Handler(&runtime.ControlRuntimeBootstrap)
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		logrus.Warnf("Received HTTP bootstrap request from %s, but embedded etcd is not enabled.", req.RemoteAddr)
		responsewriters.ErrorNegotiated(
			apierrors.NewBadRequest("etcd disabled"),
			scheme.Codecs.WithoutConversion(), schema.GroupVersion{}, resp, req,
		)
	})
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

func servingKubeletCert(server *config.Control, keyFile string, auth nodePassBootstrapper) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		nodeName, errCode, err := auth(req)
		if err != nil {
			sendError(err, resp, errCode)
			return
		}

		caCert, caKey, key, err := getCACertAndKeys(server.Runtime.ServerCA, server.Runtime.ServerCAKey, server.Runtime.ServingKubeletKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		ips := []net.IP{net.ParseIP("127.0.0.1")}

		if nodeIP := req.Header.Get(version.Program + "-Node-IP"); nodeIP != "" {
			for _, v := range strings.Split(nodeIP, ",") {
				ip := net.ParseIP(v)
				if ip == nil {
					sendError(fmt.Errorf("invalid IP address %s", ip), resp)
					return
				}
				ips = append(ips, ip)
			}
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

func clientKubeletCert(server *config.Control, keyFile string, auth nodePassBootstrapper) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}

		nodeName, errCode, err := auth(req)
		if err != nil {
			sendError(err, resp, errCode)
			return
		}

		caCert, caKey, key, err := getCACertAndKeys(server.Runtime.ClientCA, server.Runtime.ClientCAKey, server.Runtime.ClientKubeletKey)
		if err != nil {
			sendError(err, resp)
			return
		}

		cert, err := certutil.NewSignedCert(certutil.Config{
			CommonName:   "system:node:" + nodeName,
			Organization: []string{user.NodesGroup},
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

func apiserversHandler(server *config.Control) http.Handler {
	var endpointsClient coreclient.EndpointsClient
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		var endpoints []string
		if endpointsClient == nil {
			if server.Runtime.Core != nil {
				endpointsClient = server.Runtime.Core.Core().V1().Endpoints()
			}
		}
		if endpointsClient != nil {
			if endpoint, _ := endpointsClient.Get("default", "kubernetes", metav1.GetOptions{}); endpoint != nil {
				endpoints = util.GetAddresses(endpoint)
			}
		}

		resp.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(resp).Encode(endpoints); err != nil {
			logrus.Errorf("Failed to encode apiserver endpoints: %v", err)
			resp.WriteHeader(http.StatusInternalServerError)
		}
	})
}

func configHandler(server *config.Control, cfg *cmds.Server) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		// Startup hooks may read and modify cmds.Server in a goroutine, but as these are copied into
		// config.Control before the startup hooks are called, any modifications need to be sync'd back
		// into the struct before it is sent to agents.
		// At this time we don't sync all the fields, just those known to be touched by startup hooks.
		server.DisableKubeProxy = cfg.DisableKubeProxy
		resp.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(resp).Encode(server); err != nil {
			logrus.Errorf("Failed to encode agent config: %v", err)
			resp.WriteHeader(http.StatusInternalServerError)
		}
	})
}

func readyzHandler(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		code := http.StatusOK
		data := []byte("ok")
		if server.Runtime.Core == nil {
			code = http.StatusInternalServerError
			data = []byte("runtime core not ready")
		}
		resp.WriteHeader(code)
		resp.Header().Set("Content-Type", "text/plain")
		resp.Header().Set("Content-length", strconv.Itoa(len(data)))
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
	var code int
	if len(status) == 1 {
		code = status[0]
	}
	if code == 0 || code == http.StatusOK {
		code = http.StatusInternalServerError
	}
	logrus.Error(err)
	resp.WriteHeader(code)
	resp.Write([]byte(err.Error()))
}

// nodePassBootstrapper returns a node name, or http error code and error
type nodePassBootstrapper func(req *http.Request) (string, int, error)

func passwordBootstrap(ctx context.Context, config *Config) nodePassBootstrapper {
	runtime := config.ControlConfig.Runtime
	deferredNodes := map[string]bool{}
	var secretClient coreclient.SecretClient
	var mu sync.Mutex

	return nodePassBootstrapper(func(req *http.Request) (string, int, error) {
		nodeName, nodePassword, err := getNodeInfo(req)
		if err != nil {
			return "", http.StatusBadRequest, err
		}

		if secretClient == nil {
			if runtime.Core != nil {
				// initialize the client if we can
				secretClient = runtime.Core.Core().V1().Secret()
			} else if nodeName == os.Getenv("NODE_NAME") {
				// or verify the password locally and ensure a secret later
				return verifyLocalPassword(ctx, config, &mu, deferredNodes, nodeName, nodePassword)
			} else if config.ControlConfig.DisableAPIServer {
				// or defer node password verification until an apiserver joins the cluster
				return verifyRemotePassword(ctx, config, &mu, deferredNodes, nodeName, nodePassword)
			} else {
				// or reject the request until the core is ready
				return "", http.StatusServiceUnavailable, errors.New("runtime core not ready")
			}
		}

		if err := nodepassword.Ensure(secretClient, nodeName, nodePassword); err != nil {
			return "", http.StatusForbidden, err
		}

		return nodeName, http.StatusOK, nil
	})
}

func verifyLocalPassword(ctx context.Context, config *Config, mu *sync.Mutex, deferredNodes map[string]bool, nodeName, nodePassword string) (string, int, error) {
	// use same password file location that the agent creates
	nodePasswordRoot := "/"
	if config.Rootless {
		nodePasswordRoot = filepath.Join(config.ControlConfig.DataDir, "agent")
	}
	nodeConfigPath := filepath.Join(nodePasswordRoot, "etc", "rancher", "node")
	nodePasswordFile := filepath.Join(nodeConfigPath, "password")

	passBytes, err := ioutil.ReadFile(nodePasswordFile)
	if err != nil {
		return "", http.StatusInternalServerError, errors.Wrap(err, "unable to read node password file")
	}

	password := strings.TrimSpace(string(passBytes))
	if password != nodePassword {
		return "", http.StatusForbidden, errors.Wrapf(err, "unable to verify local password for node '%s'", nodeName)
	}

	mu.Lock()
	defer mu.Unlock()

	if _, ok := deferredNodes[nodeName]; !ok {
		deferredNodes[nodeName] = true
		go ensureSecret(ctx, config, nodeName, nodePassword)
		logrus.Debugf("Password verified locally for node '%s'", nodeName)
	}

	return nodeName, http.StatusOK, nil
}

func verifyRemotePassword(ctx context.Context, config *Config, mu *sync.Mutex, deferredNodes map[string]bool, nodeName, nodePassword string) (string, int, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := deferredNodes[nodeName]; !ok {
		deferredNodes[nodeName] = true
		go ensureSecret(ctx, config, nodeName, nodePassword)
		logrus.Debugf("Password verification deferred for node '%s'", nodeName)
	}

	return nodeName, http.StatusOK, nil
}

func ensureSecret(ctx context.Context, config *Config, nodeName, nodePassword string) {
	runtime := config.ControlConfig.Runtime
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
			if runtime.Core != nil {
				logrus.Debugf("Runtime core has become available, ensuring password secret for node '%s'", nodeName)
				secretClient := runtime.Core.Core().V1().Secret()
				if err := nodepassword.Ensure(secretClient, nodeName, nodePassword); err != nil {
					logrus.Warnf("Error ensuring node password secret for pre-validated node '%s': %v", nodeName, err)
				}
				return
			}
		}
	}
}
