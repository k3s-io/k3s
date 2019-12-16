// +build dqlite

package cluster

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/canonical/go-dqlite/client"
	"github.com/rancher/dynamiclistener/factory"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/dqlite"
	"github.com/rancher/kine/pkg/endpoint"
	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
)

func (c *Cluster) testClusterDB(ctx context.Context) error {
	if !c.dqliteEnabled() {
		return nil
	}

	dqlite := c.db.(*dqlite.DQLite)
	for {
		if err := dqlite.Test(ctx); err != nil {
			logrus.Infof("Failed to test dqlite connection: %v", err)
		} else {
			return nil
		}

		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Cluster) initClusterDB(ctx context.Context, l net.Listener, handler http.Handler) (net.Listener, http.Handler, error) {
	if !c.dqliteEnabled() {
		return l, handler, nil
	}

	dqlite := dqlite.New(c.config.DataDir, c.config.AdvertiseIP, c.config.AdvertisePort, func() v1.NodeController {
		if c.runtime.Core == nil {
			return nil
		}
		return c.runtime.Core.Core().V1().Node()
	})

	certs, err := toGetCerts(c.runtime)
	if err != nil {
		return nil, nil, err
	}

	handler, err = dqlite.Start(ctx, c.config.ClusterInit, c.config.ClusterReset, certs, handler)
	if err != nil {
		return nil, nil, err
	}

	if c.config.ClusterReset {
		if err := dqlite.Reset(ctx); err == nil {
			logrus.Info("Cluster reset successful, now rejoin members")
			os.Exit(0)
		} else {
			logrus.Fatalf("Cluster reset failed: %v", err)
		}
	}

	c.db = dqlite
	if !strings.HasPrefix(c.config.Datastore.Endpoint, "dqlite://") {
		c.config.Datastore = endpoint.Config{
			Endpoint: dqlite.StorageEndpoint,
		}
	}

	return l, handler, err
}

func (c *Cluster) dqliteEnabled() bool {
	stamp := filepath.Join(dqlite.GetDBDir(c.config.DataDir))
	if _, err := os.Stat(stamp); err == nil {
		return true
	}

	driver, _ := endpoint.ParseStorageEndpoint(c.config.Datastore.Endpoint)
	if driver == endpoint.DQLiteBackend {
		return true
	}

	return c.config.Datastore.Endpoint == "" && (c.config.ClusterInit || (c.config.Token != "" && c.config.JoinURL != ""))
}

func (c *Cluster) postJoin(ctx context.Context) error {
	if !c.dqliteEnabled() {
		return nil
	}

	resp, err := clientaccess.Get("/db/info", c.clientAccessInfo)
	if err != nil {
		return err
	}

	dqlite := c.db.(*dqlite.DQLite)
	var nodes []client.NodeInfo

	if err := json.Unmarshal(resp, &nodes); err != nil {
		return err
	}

	return dqlite.Join(ctx, nodes)
}

func toGetCerts(runtime *config.ControlRuntime) (*dqlite.Certs, error) {
	clientCA, _, err := factory.LoadCerts(runtime.ClientCA, runtime.ClientCAKey)
	if err != nil {
		return nil, err
	}

	ca, _, err := factory.LoadCerts(runtime.ServerCA, runtime.ServerCAKey)
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(runtime.ClientKubeAPICert, runtime.ClientKubeAPIKey)
	if err != nil {
		return nil, err
	}

	return &dqlite.Certs{
		ServerTrust: ca,
		ClientTrust: clientCA,
		ClientCert:  clientCert,
	}, nil
}
