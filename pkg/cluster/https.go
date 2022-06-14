package cluster

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/rancher/dynamiclistener/storage/file"
	"github.com/rancher/dynamiclistener/storage/kubernetes"
	"github.com/rancher/dynamiclistener/storage/memory"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilsnet "k8s.io/utils/net"
)

// newListener returns a new TCP listener and HTTP request handler using dynamiclistener.
// dynamiclistener will use the cluster's Server CA to sign the dynamically generate certificate,
// and will sync the certs into the Kubernetes datastore, with a local disk cache.
func (c *Cluster) newListener(ctx context.Context) (net.Listener, http.Handler, error) {
	if c.managedDB != nil {
		if _, err := os.Stat(etcd.ResetFile(c.config)); err == nil {
			// delete the dynamic listener file if it exists after restoration to fix restoration
			// on fresh nodes
			os.Remove(filepath.Join(c.config.DataDir, "tls/dynamic-cert.json"))
		}
	}
	ip := c.config.BindAddress
	if utilsnet.IsIPv6String(ip) {
		ip = fmt.Sprintf("[%s]", ip)
	}
	tcp, err := dynamiclistener.NewTCPListener(ip, c.config.SupervisorPort)
	if err != nil {
		return nil, nil, err
	}
	cert, key, err := factory.LoadCerts(c.config.Runtime.ServerCA, c.config.Runtime.ServerCAKey)
	if err != nil {
		return nil, nil, err
	}
	storage := tlsStorage(ctx, c.config.DataDir, c.config.Runtime)
	return wrapHandler(dynamiclistener.NewListener(tcp, storage, cert, key, dynamiclistener.Config{
		ExpirationDaysCheck: config.CertificateRenewDays,
		Organization:        []string{version.Program},
		SANs:                append(c.config.SANs, "kubernetes", "kubernetes.default", "kubernetes.default.svc", "kubernetes.default.svc."+c.config.ClusterDomain),
		CN:                  version.Program,
		TLSConfig: &tls.Config{
			ClientAuth:   tls.RequestClientCert,
			MinVersion:   c.config.TLSMinVersion,
			CipherSuites: c.config.TLSCipherSuites,
			NextProtos:   []string{"h2", "http/1.1"},
		},
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

	// Config the cluster database and allow it to add additional request handlers
	handler, err = c.initClusterDB(ctx, handler)
	if err != nil {
		return err
	}

	if c.config.EnablePProf {
		mux := mux.NewRouter()
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		mux.PathPrefix("/debug/pprof/").HandlerFunc(pprof.Index)
		mux.NotFoundHandler = handler
		handler = mux
	}

	// Create a HTTP server with the registered request handlers, using logrus for logging
	server := http.Server{
		Handler: handler,
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		server.ErrorLog = log.New(logrus.StandardLogger().Writer(), "Cluster-Http-Server ", log.LstdFlags)
	} else {
		server.ErrorLog = log.New(ioutil.Discard, "Cluster-Http-Server", 0)
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
	return kubernetes.New(ctx, func() *core.Factory {
		return runtime.Core
	}, metav1.NamespaceSystem, version.Program+"-serving", cache)
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
