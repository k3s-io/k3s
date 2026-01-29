package spegel

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/k3s-io/k3s/pkg/agent/https"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/server/auth"
	"github.com/k3s-io/k3s/pkg/util/logger"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/dynamiclistener/cert"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	leveldb "github.com/ipfs/go-ds-leveldb"
	ipfslog "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/p2p/host/peerstore/pstoreds"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spegel-org/spegel/pkg/metrics"
	"github.com/spegel-org/spegel/pkg/oci"
	"github.com/spegel-org/spegel/pkg/registry"
	"github.com/spegel-org/spegel/pkg/routing"
	"github.com/spegel-org/spegel/pkg/state"
	"k8s.io/component-base/metrics/legacyregistry"
)

// DefaultRegistry is the default instance of a Spegel distributed registry
var DefaultRegistry = &Config{
	Bootstrapper: NewSelfBootstrapper(),
	Router: func(context.Context, *config.Node) (*mux.Router, error) {
		return nil, errors.New("not implemented")
	},
}

var (
	P2pAddressAnnotation = "p2p." + version.Program + ".cattle.io/node-address"
	P2pMulAddrAnnotation = "p2p." + version.Program + ".cattle.io/node-addresses"
	P2pEnabledLabel      = "p2p." + version.Program + ".cattle.io/enabled"
	P2pPortEnv           = version.ProgramUpper + "_P2P_PORT"
	P2pEnableLatestEnv   = version.ProgramUpper + "_P2P_ENABLE_LATEST"

	resolveLatestTag = false

	wildcardRegistries = []string{"_default", "*"}

	// Agents request a list of peers when joining, and then again periodically afterwards.
	// Limit the number of concurrent peer list requests that will be served simultaneously.
	maxNonMutatingPeerInfoRequests = 20 // max concurrent get/list/watch requests
	maxMutatingPeerInfoRequests    = 0  // max concurrent other requests; not used
)

// Config holds fields for a distributed registry
type Config struct {
	ClientCAFile   string
	ClientCertFile string
	ClientKeyFile  string

	ServerCAFile   string
	ServerCertFile string
	ServerKeyFile  string

	// ExternalAddress is the address for other nodes to connect to the registry API.
	ExternalAddress string

	// InternalAddress is the address for the local containerd instance to connect to the registry API.
	InternalAddress string

	// RegistryPort is the port for the registry API.
	RegistryPort string

	// PSK is the preshared key required to join the p2p network.
	PSK []byte

	// Bootstrapper is the bootstrapper that will be used to discover p2p peers.
	Bootstrapper routing.Bootstrapper

	// HandlerFunc will be called to add the registry API handler to an existing router.
	Router https.RouterFunc

	router *routing.P2PRouter
}

// These values are not currently configurable
const (
	resolveRetries    = 3
	resolveTimeout    = time.Second * 5
	registryNamespace = "k8s.io"
	defaultRouterPort = "5001"
)

func init() {
	// ensure that spegel exposes metrics through the same registry used by Kubernetes components
	metrics.DefaultRegisterer = legacyregistry.Registerer()
	metrics.DefaultGatherer = legacyregistry.DefaultGatherer
}

// Start starts the embedded p2p router, and binds the registry API to an existing HTTP router.
func (c *Config) Start(ctx context.Context, nodeConfig *config.Node, criReadyChan <-chan struct{}) error {
	localAddr := net.JoinHostPort(c.InternalAddress, c.RegistryPort)
	// distribute images for all configured mirrors. there doesn't need to be a
	// configured endpoint, just having a key for the registry will do.
	urls := []string{}
	registries := []string{}
	for host := range nodeConfig.AgentConfig.Registry.Mirrors {
		if host == localAddr {
			continue
		}
		if _, err := url.Parse("https://" + host); err != nil || docker.IsLocalhost(host) {
			logrus.Errorf("Distributed registry mirror skipping invalid registry: %s", host)
		} else if slices.Contains(wildcardRegistries, host) {
			urls = append(urls, host)
			registries = append(registries, host)
		} else {
			urls = append(urls, "https://"+host)
			registries = append(registries, host)
		}
	}

	if len(registries) == 0 {
		logrus.Errorf("Not starting distributed registry mirror: no registries configured for distributed mirroring")
		return nil
	}

	filters := []oci.Filter{}
	regFilter, err := oci.FilterForMirroredRegistries(urls)
	if err != nil {
		return err
	}
	if regFilter != nil {
		filters = append(filters, *regFilter)
	}
	if !resolveLatestTag {
		filters = append(filters, oci.RegexFilter{Regex: regexp.MustCompile(`:latest$`)})
	}

	logrus.Infof("Starting distributed registry mirror at https://%s:%s/v2 for registries %v",
		c.ExternalAddress, c.RegistryPort, registries)

	// set up the various logging logging frameworks
	ctx = logr.NewContext(ctx, logger.NewLogrusSink(nil).AsLogr().WithName("spegel"))
	level := ipfslog.LevelInfo
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		level = ipfslog.LevelDebug
	}
	ipfslog.SetAllLoggers(level)

	// Get containerd client
	caCert, err := os.ReadFile(c.ServerCAFile)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to read server CA")
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)

	clientCert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientKeyFile)
	if err != nil {
		return err
	}

	clientOpts := []oci.ClientOption{
		oci.WithTLS(certPool, []tls.Certificate{clientCert}),
	}
	ociClient, err := oci.NewClient(clientOpts...)
	if err != nil {
		return err
	}

	storeOpts := []oci.ContainerdOption{
		oci.WithContentPath(filepath.Join(nodeConfig.Containerd.Root, "io.containerd.content.v1.content")),
	}
	ociStore, err := NewDeferredContainerd(ctx, nodeConfig.Containerd.Address, registryNamespace, storeOpts...)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to create OCI store")
	}

	// create or load persistent private key
	keyFile := filepath.Join(nodeConfig.Containerd.Opt, "peer.key")
	keyBytes, _, err := cert.LoadOrGenerateKeyFile(keyFile, false)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to load or generate p2p private key")
	}
	privKey, err := cert.ParsePrivateKeyPEM(keyBytes)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to parse p2p private key")
	}
	p2pKey, _, err := crypto.KeyPairFromStdKey(privKey)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to convert p2p private key")
	}

	// create a peerstore to allow persisting nodes across restarts
	peerFile := filepath.Join(nodeConfig.Containerd.Opt, "peerstore.db")
	ds, err := leveldb.NewDatastore(peerFile, nil)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to create peerstore datastore")
	}
	ps, err := pstoreds.NewPeerstore(ctx, ds, pstoreds.DefaultOpts())
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to create peerstore")
	}

	// get latest tag configuration override
	if env := os.Getenv(P2pEnableLatestEnv); env != "" {
		if b, err := strconv.ParseBool(env); err != nil {
			logrus.Warnf("Invalid %s value; using default %v", P2pEnableLatestEnv, resolveLatestTag)
		} else {
			resolveLatestTag = b
		}
	}

	// get port and start p2p router
	routerPort := defaultRouterPort
	if env := os.Getenv(P2pPortEnv); env != "" {
		if i, err := strconv.Atoi(env); i == 0 || err != nil {
			logrus.Warnf("Invalid %s value; using default %v", P2pPortEnv, defaultRouterPort)
		} else {
			routerPort = env
		}
	}
	routerAddr := net.JoinHostPort(c.ExternalAddress, routerPort)

	logrus.Infof("Starting distributed registry P2P node at %s", routerAddr)
	opts := []routing.P2PRouterOption{
		routing.WithLogConnectivityErrors(false),
		routing.WithLibP2POptions(
			libp2p.Identity(p2pKey),
			libp2p.Peerstore(ps),
			libp2p.PrivateNetwork(c.PSK),
		),
	}
	c.router, err = routing.NewP2PRouter(ctx, routerAddr, NewNotSelfBootstrapper(c.Bootstrapper), c.RegistryPort, opts...)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to create P2P router")
	}
	go c.router.Run(ctx)

	metrics.Register()
	registryOpts := []registry.RegistryOption{
		registry.WithRegistryFilters(filters),
		registry.WithResolveRetries(resolveRetries),
		registry.WithResolveTimeout(resolveTimeout),
		registry.WithOCIClient(ociClient),
	}
	reg, err := registry.NewRegistry(ociStore, c.router, registryOpts...)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to create embedded registry")
	}
	regSvr := &http.Server{
		Addr:    ":" + c.RegistryPort,
		Handler: reg.Handler(logr.FromContextOrDiscard(ctx)),
	}

	trackerOpts := []state.TrackerOption{
		state.WithRegistryFilters(filters),
	}

	// Track images available in containerd and publish via p2p router
	go func() {
		defer ociStore.Close()
		<-criReadyChan
		for {
			logrus.Debug("Starting embedded registry image state tracker")
			if err := ociStore.Start(); err != nil {
				logrus.Errorf("Failed to start deferred OCI store: %v", err)
			}
			err := state.Track(ctx, ociStore, c.router, trackerOpts...)
			if err != nil && errors.Is(err, context.Canceled) {
				return
			}
			logrus.Errorf("Embedded registry image state tracker exited: %v", err)
			time.Sleep(time.Second)
		}
	}()

	mRouter, err := c.Router(ctx, nodeConfig)
	if err != nil {
		return err
	}
	mRouter.PathPrefix("/v2").Handler(regSvr.Handler)
	sRouter := mRouter.PathPrefix("/v1-{program}/p2p").Subrouter()
	sRouter.Use(auth.MaxInFlight(maxNonMutatingPeerInfoRequests, maxMutatingPeerInfoRequests))
	sRouter.Handle("", c.peerInfo())

	// Wait up to 5 seconds for the p2p network to find peers.
	if err := wait.PollUntilContextTimeout(ctx, time.Second, resolveTimeout, true, func(ctx context.Context) (bool, error) {
		ready, _ := c.router.Ready(ctx)
		return ready, nil
	}); err != nil {
		logrus.Warn("Failed to wait for distributed registry to become ready, will retry in the background")
	}
	return nil
}

func (c *Config) Ready(ctx context.Context) (bool, error) {
	if c.router == nil {
		return false, nil
	}
	return c.router.Ready(ctx)
}

// peerInfo sends a peer address retrieved from the bootstrapper via HTTP
func (c *Config) peerInfo() http.HandlerFunc {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		info, err := c.Bootstrapper.Get(req.Context())
		if err != nil {
			http.Error(resp, err.Error(), http.StatusInternalServerError)
			return
		}

		addrs := []string{}
		for _, ai := range info {
			for _, ma := range ai.Addrs {
				addrs = append(addrs, fmt.Sprintf("%s/p2p/%s", ma, ai.ID))
			}
		}

		if len(addrs) == 0 {
			http.Error(resp, "no peer addresses available", http.StatusServiceUnavailable)
			return
		}

		client, _, _ := net.SplitHostPort(req.RemoteAddr)
		if req.Header.Get("Accept") == "application/json" {
			b, err := json.Marshal(addrs)
			if err != nil {
				http.Error(resp, err.Error(), http.StatusInternalServerError)
				return
			}
			logrus.Debugf("Serving p2p peer addrs %v to client at %s", addrs, client)
			resp.Header().Set("Content-Type", "application/json")
			resp.WriteHeader(http.StatusOK)
			resp.Write(b)
			return
		}

		logrus.Debugf("Serving p2p peer addr %v to client at %s", addrs[0], client)
		resp.Header().Set("Content-Type", "text/plain")
		resp.WriteHeader(http.StatusOK)
		resp.Write([]byte(addrs[0]))
	})
}
