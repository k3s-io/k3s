package endpoint

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/k3s-io/kine/pkg/drivers/dqlite"
	"github.com/k3s-io/kine/pkg/drivers/generic"
	"github.com/k3s-io/kine/pkg/drivers/mysql"
	"github.com/k3s-io/kine/pkg/drivers/pgsql"
	"github.com/k3s-io/kine/pkg/drivers/sqlite"
	"github.com/k3s-io/kine/pkg/server"
	"github.com/k3s-io/kine/pkg/tls"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	"go.etcd.io/etcd/server/v3/embed"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

const (
	KineSocket      = "unix://kine.sock"
	SQLiteBackend   = "sqlite"
	DQLiteBackend   = "dqlite"
	ETCDBackend     = "etcd3"
	MySQLBackend    = "mysql"
	PostgresBackend = "postgres"
)

type Config struct {
	GRPCServer           *grpc.Server
	Listener             string
	Endpoint             string
	ConnectionPoolConfig generic.ConnectionPoolConfig
	ServerTLSConfig      tls.Config
	BackendTLSConfig     tls.Config
}

type ETCDConfig struct {
	Endpoints   []string
	TLSConfig   tls.Config
	LeaderElect bool
}

func Listen(ctx context.Context, config Config) (ETCDConfig, error) {
	driver, dsn := ParseStorageEndpoint(config.Endpoint)
	if driver == ETCDBackend {
		return ETCDConfig{
			Endpoints:   strings.Split(config.Endpoint, ","),
			TLSConfig:   config.BackendTLSConfig,
			LeaderElect: true,
		}, nil
	}

	leaderelect, backend, err := getKineStorageBackend(ctx, driver, dsn, config)
	if err != nil {
		return ETCDConfig{}, errors.Wrap(err, "building kine")
	}

	if err := backend.Start(ctx); err != nil {
		return ETCDConfig{}, errors.Wrap(err, "starting kine backend")
	}

	// set up GRPC server and register services
	b := server.New(backend, endpointScheme(config))
	grpcServer, err := grpcServer(config)
	if err != nil {
		return ETCDConfig{}, errors.Wrap(err, "creating GRPC server")
	}
	b.Register(grpcServer)

	// set up HTTP server with basic mux
	httpServer := httpServer()

	// Create raw listener and wrap in cmux for protocol switching
	listener, err := createListener(config)
	if err != nil {
		return ETCDConfig{}, errors.Wrap(err, "creating listener")
	}
	m := cmux.New(listener)

	if config.ServerTLSConfig.CertFile != "" && config.ServerTLSConfig.KeyFile != "" {
		// If using TLS, wrap handler in GRPC/HTTP switching handler and serve TLS
		httpServer.Handler = grpcHandlerFunc(grpcServer, httpServer.Handler)
		anyl := m.Match(cmux.Any())
		go func() {
			if err := httpServer.ServeTLS(anyl, config.ServerTLSConfig.CertFile, config.ServerTLSConfig.KeyFile); err != nil {
				logrus.Errorf("Kine TLS server shutdown: %v", err)
			}
		}()
	} else {
		// If using plaintext, use cmux matching for GRPC/HTTP switching
		grpcl := m.Match(cmux.HTTP2())
		go func() {
			if err := grpcServer.Serve(grpcl); err != nil {
				logrus.Errorf("Kine GRPC server shutdown: %v", err)
			}
		}()
		httpl := m.Match(cmux.HTTP1())
		go func() {
			if err := httpServer.Serve(httpl); err != nil {
				logrus.Errorf("Kine HTTP server shutdown: %v", err)
			}
		}()
	}

	go func() {
		if err := m.Serve(); err != nil {
			logrus.Errorf("Kine listener shutdown: %v", err)
			grpcServer.Stop()
		}
	}()

	endpoint := endpointURL(config, listener)
	logrus.Infof("Kine available at %s", endpoint)

	return ETCDConfig{
		LeaderElect: leaderelect,
		Endpoints:   []string{endpoint},
		TLSConfig:   tls.Config{},
	}, nil
}

// endpointURL returns a URI string suitable for use as a local etcd endpoint.
// For TCP sockets, it is assumed that the port can be reached via the loopback address.
func endpointURL(config Config, listener net.Listener) string {
	scheme := endpointScheme(config)
	address := listener.Addr().String()
	if !strings.HasPrefix(scheme, "unix") {
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			logrus.Warnf("failed to get listener port: %v", err)
			port = "2379"
		}
		address = "127.0.0.1:" + port
	}

	return scheme + "://" + address
}

// endpointScheme returns the URI scheme for the listener specified by the configuration.
func endpointScheme(config Config) string {
	if config.Listener == "" {
		config.Listener = KineSocket
	}

	network, _ := networkAndAddress(config.Listener)
	if network != "unix" {
		network = "http"
	}

	if config.ServerTLSConfig.CertFile != "" && config.ServerTLSConfig.KeyFile != "" {
		// yes, etcd supports the "unixs" scheme for TLS over unix sockets
		network += "s"
	}

	return network
}

// createListener returns a listener bound to the requested protocol and address.
func createListener(config Config) (ret net.Listener, rerr error) {
	if config.Listener == "" {
		config.Listener = KineSocket
	}
	network, address := networkAndAddress(config.Listener)

	if network == "unix" {
		if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
			logrus.Warnf("failed to remove socket %s: %v", address, err)
		}
		defer func() {
			if err := os.Chmod(address, 0600); err != nil {
				rerr = err
			}
		}()
	} else {
		network = "tcp"
	}

	return net.Listen(network, address)
}

// grpcServer returns either a preconfigured GRPC server, or builds a new GRPC
// server using upstream keepalive defaults plus the local Server TLS configuration.
func grpcServer(config Config) (*grpc.Server, error) {
	if config.GRPCServer != nil {
		return config.GRPCServer, nil
	}

	gopts := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             embed.DefaultGRPCKeepAliveMinTime,
			PermitWithoutStream: false,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    embed.DefaultGRPCKeepAliveInterval,
			Timeout: embed.DefaultGRPCKeepAliveTimeout,
		}),
	}

	if config.ServerTLSConfig.CertFile != "" && config.ServerTLSConfig.KeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(config.ServerTLSConfig.CertFile, config.ServerTLSConfig.KeyFile)
		if err != nil {
			return nil, err
		}
		gopts = append(gopts, grpc.Creds(creds))
	}

	return grpc.NewServer(gopts...), nil
}

// getKineStorageBackend parses the driver string, and returns a bool
// indicating whether the backend requires leader election, and a suitable
// backend datastore connection.
func getKineStorageBackend(ctx context.Context, driver, dsn string, cfg Config) (bool, server.Backend, error) {
	var (
		backend     server.Backend
		leaderElect = true
		err         error
	)
	switch driver {
	case SQLiteBackend:
		leaderElect = false
		backend, err = sqlite.New(ctx, dsn, cfg.ConnectionPoolConfig)
	case DQLiteBackend:
		backend, err = dqlite.New(ctx, dsn, cfg.ConnectionPoolConfig)
	case PostgresBackend:
		backend, err = pgsql.New(ctx, dsn, cfg.BackendTLSConfig, cfg.ConnectionPoolConfig)
	case MySQLBackend:
		backend, err = mysql.New(ctx, dsn, cfg.BackendTLSConfig, cfg.ConnectionPoolConfig)
	default:
		return false, nil, fmt.Errorf("storage backend is not defined")
	}

	return leaderElect, backend, err
}

// ParseStorageEndpoint returns the driver name and endpoint string from a datastore endpoint URL.
func ParseStorageEndpoint(storageEndpoint string) (string, string) {
	network, address := networkAndAddress(storageEndpoint)
	switch network {
	case "":
		return SQLiteBackend, ""
	case "http":
		fallthrough
	case "https":
		return ETCDBackend, address
	}
	return network, address
}

// networkAndAddress crudely splits a URL string into network (scheme) and address,
// where the address includes everything after the scheme/authority separator.
func networkAndAddress(str string) (string, string) {
	parts := strings.SplitN(str, "://", 2)
	if len(parts) > 1 {
		return parts[0], parts[1]
	}
	return "", parts[0]
}
