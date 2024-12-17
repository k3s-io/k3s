package cluster

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/rancher/dynamiclistener/storage/file"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	"github.com/rancher/dynamiclistener/storage/memory"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newListener returns a new TCP listener and HTTP request handler using dynamiclistener.
// dynamiclistener will use the cluster's Server CA to sign the dynamically generate certificate,
// and will sync the certs into the Kubernetes datastore, with a local disk cache.
func (c *Cluster) newListener(ctx context.Context) (net.Listener, http.Handler, error) {
	if c.managedDB != nil {
		resetDone, err := c.managedDB.IsReset()
		if err != nil {
			return nil, nil, err
		}
		if resetDone {
			// delete the dynamic listener TLS secret cache after restoring,
			// to ensure that dynamiclistener doesn't sync the old secret over the top
			// of whatever was just restored.
			os.Remove(filepath.Join(c.config.DataDir, "tls/dynamic-cert.json"))
		}
	}
	tcp, err := util.ListenWithLoopback(ctx, c.config.BindAddress, strconv.Itoa(c.config.SupervisorPort))
	if err != nil {
		return nil, nil, err
	}
	certs, key, err := factory.LoadCertsChain(c.config.Runtime.ServerCA, c.config.Runtime.ServerCAKey)
	if err != nil {
		return nil, nil, err
	}
	c.config.SANs = append(c.config.SANs, "kubernetes", "kubernetes.default", "kubernetes.default.svc", "kubernetes.default.svc."+c.config.ClusterDomain)
	if c.config.SANSecurity {
		c.config.Runtime.ClusterControllerStarts["server-cn-filter"] = func(ctx context.Context) {
			registerAddressHandlers(ctx, c)
		}
	}
	storage := tlsStorage(ctx, c.config.DataDir, c.config.Runtime)
	return wrapHandler(dynamiclistener.NewListenerWithChain(tcp, storage, certs, key, dynamiclistener.Config{
		ExpirationDaysCheck: config.CertificateRenewDays,
		Organization:        []string{version.Program},
		SANs:                c.config.SANs,
		CN:                  version.Program,
		TLSConfig: &tls.Config{
			ClientAuth:   tls.RequestClientCert,
			MinVersion:   c.config.TLSMinVersion,
			CipherSuites: c.config.TLSCipherSuites,
			NextProtos:   []string{"h2", "http/1.1"},
		},
		FilterCN: c.filterCN,
		RegenerateCerts: func() bool {
			const regenerateDynamicListenerFile = "dynamic-cert-regenerate"
			dynamicListenerRegenFilePath := filepath.Join(c.config.DataDir, "tls", regenerateDynamicListenerFile)
			if _, err := os.Stat(dynamicListenerRegenFilePath); err == nil {
				os.Remove(dynamicListenerRegenFilePath)
				return true
			}
			return false
		},
	}))
}

func (c *Cluster) filterCN(cn ...string) []string {
	if c.cnFilterFunc != nil {
		return c.cnFilterFunc(cn...)
	}
	return cn
}

// initClusterAndHTTPS sets up the dynamic tls listener, request router,
// and cluster database. Once the database is up, it starts the supervisor http server.
func (c *Cluster) initClusterAndHTTPS(ctx context.Context) error {
	// Set up dynamiclistener TLS listener and request handler
	listener, handler, err := c.newListener(ctx)
	if err != nil {
		return err
	}

	// Get the base request handler
	handler, err = c.getHandler(handler)
	if err != nil {
		return err
	}

	// Register database request handlers and controller callbacks
	handler, err = c.registerDBHandlers(handler)
	if err != nil {
		return err
	}

	// Create a HTTP server with the registered request handlers, using logrus for logging
	server := http.Server{
		Handler: handler,
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		server.ErrorLog = log.New(logrus.StandardLogger().Writer(), "Cluster-Http-Server ", log.LstdFlags)
	} else {
		server.ErrorLog = log.New(io.Discard, "Cluster-Http-Server", 0)
	}

	// Start the supervisor http server on the tls listener
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Fatalf("server stopped: %v", err)
		}
	}()

	// Shutdown the http server when the context is closed
	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	return nil
}

// tlsStorage creates an in-memory cache for dynamiclistener's certificate, backed by a file on disk
// and the Kubernetes datastore.
func tlsStorage(ctx context.Context, dataDir string, runtime *config.ControlRuntime) dynamiclistener.TLSStorage {
	fileStorage := file.New(filepath.Join(dataDir, "tls/dynamic-cert.json"))
	cache := memory.NewBacked(fileStorage)
	coreGetter := func() *core.Factory {
		if coreFactory, ok := runtime.Core.(*core.Factory); ok {
			return coreFactory
		}
		return nil
	}
	return kubernetes.New(ctx, coreGetter, metav1.NamespaceSystem, version.Program+"-serving", cache)
}

// wrapHandler wraps the dynamiclistener request handler, adding a User-Agent value to
// CONNECT requests that will prevent DynamicListener from adding the request's Host
// header to the SAN list.  CONNECT requests set the Host header to the target of the
// proxy connection, so it is not correct to add this value to the certificate.  It would
// be nice if we could do this with with the FilterCN callback, but unfortunately that
// callback does not offer access to the request that triggered the change.
func wrapHandler(listener net.Listener, handler http.Handler, err error) (net.Listener, http.Handler, error) {
	return listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			r.Header.Add("User-Agent", "mozilla")
		}
		handler.ServeHTTP(w, r)
	}), err
}
