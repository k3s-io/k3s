package server

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/nodepassword"
	"github.com/k3s-io/k3s/pkg/server/auth"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
)

const (
	staticURL = "/static/"
)

var (
	identifier = nodeidentifier.NewDefaultNodeIdentifier()
)

func router(ctx context.Context, config *Config, cfg *cmds.Server) http.Handler {
	serverConfig := &config.ControlConfig
	nodeAuth := passwordBootstrap(ctx, config)

	prefix := "/v1-" + version.Program
	authed := mux.NewRouter().SkipClean(true)
	authed.Use(auth.HasRole(serverConfig, version.Program+":agent", user.NodesGroup, bootstrapapi.BootstrapDefaultGroup))
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
	nodeAuthed.Use(auth.HasRole(serverConfig, user.NodesGroup))
	nodeAuthed.Path(prefix + "/connect").Handler(serverConfig.Runtime.Tunnel)

	serverAuthed := mux.NewRouter().SkipClean(true)
	serverAuthed.NotFoundHandler = nodeAuthed
	serverAuthed.Use(auth.HasRole(serverConfig, version.Program+":server"))
	serverAuthed.Path(prefix + "/encrypt/status").Handler(encryptionStatusHandler(serverConfig))
	serverAuthed.Path(prefix + "/encrypt/config").Handler(encryptionConfigHandler(ctx, serverConfig))
	serverAuthed.Path(prefix + "/cert/cacerts").Handler(caCertReplaceHandler(serverConfig))
	serverAuthed.Path(prefix + "/server-bootstrap").Handler(bootstrapHandler(serverConfig.Runtime))
	serverAuthed.Path(prefix + "/token").Handler(tokenRequestHandler(ctx, serverConfig))

	systemAuthed := mux.NewRouter().SkipClean(true)
	systemAuthed.NotFoundHandler = serverAuthed
	systemAuthed.MethodNotAllowedHandler = serverAuthed
	systemAuthed.Use(auth.HasRole(serverConfig, user.SystemPrivilegedGroup))
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
			util.SendError(util.ErrAPINotReady, resp, req, http.StatusServiceUnavailable)
		}
	})
}

func apiserverDisabled() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		util.SendError(util.ErrAPIDisabled, resp, req, http.StatusServiceUnavailable)
	})
}

func bootstrapHandler(runtime *config.ControlRuntime) http.Handler {
	if runtime.HTTPBootstrap {
		return bootstrap.Handler(&runtime.ControlRuntimeBootstrap)
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		logrus.Warnf("Received HTTP bootstrap request from %s, but embedded etcd is not enabled.", req.RemoteAddr)
		util.SendError(errors.New("etcd disabled"), resp, req, http.StatusBadRequest)
	})
}

func cacerts(serverCA string) http.Handler {
	var ca []byte
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if ca == nil {
			var err error
			ca, err = os.ReadFile(serverCA)
			if err != nil {
				util.SendError(err, resp, req)
				return
			}
		}
		resp.Header().Set("content-type", "text/plain")
		resp.Write(ca)
	})
}

func getNodeInfo(req *http.Request) (*nodeInfo, error) {
	user, ok := request.UserFrom(req.Context())
	if !ok {
		return nil, errors.New("auth user not set")
	}

	nodeName := req.Header.Get(version.Program + "-Node-Name")
	if nodeName == "" {
		return nil, errors.New("node name not set")
	}

	nodePassword := req.Header.Get(version.Program + "-Node-Password")
	if nodePassword == "" {
		return nil, errors.New("node password not set")
	}

	return &nodeInfo{
		Name:     strings.ToLower(nodeName),
		Password: nodePassword,
		User:     user,
	}, nil
}

func getCACertAndKeys(caCertFile, caKeyFile, signingKeyFile string) ([]*x509.Certificate, crypto.Signer, crypto.Signer, error) {
	keyBytes, err := os.ReadFile(signingKeyFile)
	if err != nil {
		return nil, nil, nil, err
	}

	key, err := certutil.ParsePrivateKeyPEM(keyBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	caKeyBytes, err := os.ReadFile(caKeyFile)
	if err != nil {
		return nil, nil, nil, err
	}

	caKey, err := certutil.ParsePrivateKeyPEM(caKeyBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	caBytes, err := os.ReadFile(caCertFile)
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
		nodeName, errCode, err := auth(req)
		if err != nil {
			util.SendError(err, resp, req, errCode)
			return
		}

		caCerts, caKey, key, err := getCACertAndKeys(server.Runtime.ServerCA, server.Runtime.ServerCAKey, server.Runtime.ServingKubeletKey)
		if err != nil {
			util.SendError(err, resp, req)
			return
		}

		ips := []net.IP{net.ParseIP("127.0.0.1")}

		if nodeIP := req.Header.Get(version.Program + "-Node-IP"); nodeIP != "" {
			for _, v := range strings.Split(nodeIP, ",") {
				ip := net.ParseIP(v)
				if ip == nil {
					util.SendError(fmt.Errorf("invalid node IP address %s", ip), resp, req)
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
		}, key, caCerts[0], caKey)
		if err != nil {
			util.SendError(err, resp, req)
			return
		}

		keyBytes, err := os.ReadFile(keyFile)
		if err != nil {
			http.Error(resp, err.Error(), http.StatusInternalServerError)
			return
		}

		resp.Write(util.EncodeCertsPEM(cert, caCerts))
		resp.Write(keyBytes)
	})
}

func clientKubeletCert(server *config.Control, keyFile string, auth nodePassBootstrapper) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		nodeName, errCode, err := auth(req)
		if err != nil {
			util.SendError(err, resp, req, errCode)
			return
		}

		caCerts, caKey, key, err := getCACertAndKeys(server.Runtime.ClientCA, server.Runtime.ClientCAKey, server.Runtime.ClientKubeletKey)
		if err != nil {
			util.SendError(err, resp, req)
			return
		}

		cert, err := certutil.NewSignedCert(certutil.Config{
			CommonName:   "system:node:" + nodeName,
			Organization: []string{user.NodesGroup},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}, key, caCerts[0], caKey)
		if err != nil {
			util.SendError(err, resp, req)
			return
		}

		keyBytes, err := os.ReadFile(keyFile)
		if err != nil {
			http.Error(resp, err.Error(), http.StatusInternalServerError)
			return
		}

		resp.Write(util.EncodeCertsPEM(cert, caCerts))
		resp.Write(keyBytes)
	})
}

func fileHandler(fileName ...string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "text/plain")

		if len(fileName) == 1 {
			http.ServeFile(resp, req, fileName[0])
			return
		}

		for _, f := range fileName {
			bytes, err := os.ReadFile(f)
			if err != nil {
				util.SendError(errors.Wrapf(err, "failed to read %s", f), resp, req, http.StatusInternalServerError)
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
			util.SendError(errors.Wrap(err, "failed to encode apiserver endpoints"), resp, req, http.StatusInternalServerError)
		}
	})
}

func configHandler(server *config.Control, cfg *cmds.Server) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		// Startup hooks may read and modify cmds.Server in a goroutine, but as these are copied into
		// config.Control before the startup hooks are called, any modifications need to be sync'd back
		// into the struct before it is sent to agents.
		// At this time we don't sync all the fields, just those known to be touched by startup hooks.
		server.DisableKubeProxy = cfg.DisableKubeProxy
		resp.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(resp).Encode(server); err != nil {
			util.SendError(errors.Wrap(err, "failed to encode agent config"), resp, req, http.StatusInternalServerError)
		}
	})
}

func readyzHandler(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if server.Runtime.Core == nil {
			util.SendError(util.ErrCoreNotReady, resp, req, http.StatusServiceUnavailable)
			return
		}
		data := []byte("ok")
		resp.WriteHeader(http.StatusOK)
		resp.Header().Set("Content-Type", "text/plain")
		resp.Header().Set("Content-Length", strconv.Itoa(len(data)))
		resp.Write(data)
	})
}

func ping() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		data := []byte("pong")
		resp.WriteHeader(http.StatusOK)
		resp.Header().Set("Content-Type", "text/plain")
		resp.Header().Set("Content-Length", strconv.Itoa(len(data)))
		resp.Write(data)
	})
}

func serveStatic(urlPrefix, staticDir string) http.Handler {
	return http.StripPrefix(urlPrefix, http.FileServer(http.Dir(staticDir)))
}

// nodePassBootstrapper returns a node name, or http error code and error
type nodePassBootstrapper func(req *http.Request) (string, int, error)

// nodeInfo contains information on the requesting node, derived from auth creds
// and request headers.
type nodeInfo struct {
	Name     string
	Password string
	User     user.Info
}

func passwordBootstrap(ctx context.Context, config *Config) nodePassBootstrapper {
	runtime := config.ControlConfig.Runtime
	deferredNodes := map[string]bool{}
	var secretClient coreclient.SecretController
	var nodeClient coreclient.NodeController
	var mu sync.Mutex

	return nodePassBootstrapper(func(req *http.Request) (string, int, error) {
		node, err := getNodeInfo(req)
		if err != nil {
			return "", http.StatusBadRequest, err
		}

		nodeName, isNodeAuth := identifier.NodeIdentity(node.User)
		if isNodeAuth && nodeName != node.Name {
			return "", http.StatusBadRequest, errors.New("header node name does not match auth node name")
		}

		if secretClient == nil || nodeClient == nil {
			if runtime.Core != nil {
				// initialize the client if we can
				secretClient = runtime.Core.Core().V1().Secret()
				nodeClient = runtime.Core.Core().V1().Node()
			} else if node.Name == os.Getenv("NODE_NAME") {
				// If we're verifying our own password, verify it locally and ensure a secret later.
				return verifyLocalPassword(ctx, config, &mu, deferredNodes, node)
			} else if config.ControlConfig.DisableAPIServer && !isNodeAuth {
				// If we're running on an etcd-only node, and the request didn't use Node Identity auth,
				// defer node password verification until an apiserver joins the cluster.
				return verifyRemotePassword(ctx, config, &mu, deferredNodes, node)
			} else {
				// Otherwise, reject the request until the core is ready.
				return "", http.StatusServiceUnavailable, util.ErrCoreNotReady
			}
		}

		// verify that the node exists, if using Node Identity auth
		if err := verifyNode(ctx, nodeClient, node); err != nil {
			return "", http.StatusUnauthorized, err
		}

		// verify that the node password secret matches, or create it if it does not
		if err := nodepassword.Ensure(secretClient, node.Name, node.Password); err != nil {
			// if the verification failed, reject the request
			if errors.Is(err, nodepassword.ErrVerifyFailed) {
				return "", http.StatusForbidden, err
			}
			// If verification failed due to an error creating the node password secret, allow
			// the request, but retry verification until the outage is resolved.  This behavior
			// allows nodes to join the cluster during outages caused by validating webhooks
			// blocking secret creation - if the outage requires new nodes to join in order to
			// run the webhook pods, we must fail open here to resolve the outage.
			return verifyRemotePassword(ctx, config, &mu, deferredNodes, node)
		}

		return node.Name, http.StatusOK, nil
	})
}

func verifyLocalPassword(ctx context.Context, config *Config, mu *sync.Mutex, deferredNodes map[string]bool, node *nodeInfo) (string, int, error) {
	// do not attempt to verify the node password if the local host is not running an agent and does not have a node resource.
	if config.DisableAgent {
		return node.Name, http.StatusOK, nil
	}

	// use same password file location that the agent creates
	nodePasswordRoot := "/"
	if config.ControlConfig.Rootless {
		nodePasswordRoot = filepath.Join(path.Dir(config.ControlConfig.DataDir), "agent")
	}
	nodeConfigPath := filepath.Join(nodePasswordRoot, "etc", "rancher", "node")
	nodePasswordFile := filepath.Join(nodeConfigPath, "password")

	passBytes, err := os.ReadFile(nodePasswordFile)
	if err != nil {
		return "", http.StatusInternalServerError, errors.Wrap(err, "unable to read node password file")
	}

	passHash, err := nodepassword.Hasher.CreateHash(strings.TrimSpace(string(passBytes)))
	if err != nil {
		return "", http.StatusInternalServerError, errors.Wrap(err, "unable to hash node password file")
	}

	if err := nodepassword.Hasher.VerifyHash(passHash, node.Password); err != nil {
		return "", http.StatusForbidden, errors.Wrap(err, "unable to verify local node password")
	}

	mu.Lock()
	defer mu.Unlock()

	if _, ok := deferredNodes[node.Name]; !ok {
		deferredNodes[node.Name] = true
		go ensureSecret(ctx, config, node)
		logrus.Infof("Password verified locally for node %s", node.Name)
	}

	return node.Name, http.StatusOK, nil
}

func verifyRemotePassword(ctx context.Context, config *Config, mu *sync.Mutex, deferredNodes map[string]bool, node *nodeInfo) (string, int, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := deferredNodes[node.Name]; !ok {
		deferredNodes[node.Name] = true
		go ensureSecret(ctx, config, node)
		logrus.Infof("Password verification deferred for node %s", node.Name)
	}

	return node.Name, http.StatusOK, nil
}

func verifyNode(ctx context.Context, nodeClient coreclient.NodeController, node *nodeInfo) error {
	if nodeName, isNodeAuth := identifier.NodeIdentity(node.User); isNodeAuth {
		if _, err := nodeClient.Cache().Get(nodeName); err != nil {
			return errors.Wrap(err, "unable to verify node identity")
		}
	}
	return nil
}

func ensureSecret(ctx context.Context, config *Config, node *nodeInfo) {
	runtime := config.ControlConfig.Runtime
	_ = wait.PollUntilContextCancel(ctx, time.Second*5, true, func(ctx context.Context) (bool, error) {
		if runtime.Core != nil {
			secretClient := runtime.Core.Core().V1().Secret()
			// This is consistent with events attached to the node generated by the kubelet
			// https://github.com/kubernetes/kubernetes/blob/612130dd2f4188db839ea5c2dea07a96b0ad8d1c/pkg/kubelet/kubelet.go#L479-L485
			nodeRef := &corev1.ObjectReference{
				Kind:      "Node",
				Name:      node.Name,
				UID:       types.UID(node.Name),
				Namespace: "",
			}
			if err := nodepassword.Ensure(secretClient, node.Name, node.Password); err != nil {
				runtime.Event.Eventf(nodeRef, corev1.EventTypeWarning, "NodePasswordValidationFailed", "Deferred node password secret validation failed: %v", err)
				// Return true to stop polling if the password verification failed; only retry on secret creation errors.
				return errors.Is(err, nodepassword.ErrVerifyFailed), nil
			}
			runtime.Event.Event(nodeRef, corev1.EventTypeNormal, "NodePasswordValidationComplete", "Deferred node password secret validation complete")
			return true, nil
		}
		return false, nil
	})
}
