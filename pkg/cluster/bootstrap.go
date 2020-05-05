package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

func (c *Cluster) Bootstrap(ctx context.Context) error {
	if err := c.assignManagedDriver(ctx); err != nil {
		return err
	}

	runBootstrap, err := c.shouldBootstrapLoad()
	if err != nil {
		return err
	}
	c.shouldBootstrap = runBootstrap

	if runBootstrap {
		if err := c.bootstrap(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *Cluster) shouldBootstrapLoad() (bool, error) {
	if c.managedDB != nil {
		c.runtime.HTTPBootstrap = true
		if c.config.JoinURL == "" {
			return false, nil
		}

		token, err := clientaccess.NormalizeAndValidateTokenForUser(c.config.JoinURL, c.config.Token, "server")
		if err != nil {
			return false, err
		}

		info, err := clientaccess.ParseAndValidateToken(c.config.JoinURL, token)
		if err != nil {
			return false, err
		}
		c.clientAccessInfo = info
	}

	stamp := c.bootstrapStamp()
	if _, err := os.Stat(stamp); err == nil {
		logrus.Info("Cluster bootstrap already complete")
		return false, nil
	}

	if c.managedDB != nil && c.config.Token == "" {
		return false, fmt.Errorf("K3S_TOKEN is required to join a cluster")
	}

	return true, nil
}

func (c *Cluster) bootstrapped() error {
	if err := os.MkdirAll(filepath.Dir(c.bootstrapStamp()), 0700); err != nil {
		return err
	}

	if _, err := os.Stat(c.bootstrapStamp()); err == nil {
		return nil
	}

	f, err := os.Create(c.bootstrapStamp())
	if err != nil {
		return err
	}

	return f.Close()
}

func (c *Cluster) httpBootstrap() error {
	content, err := clientaccess.Get("/v1-"+version.Program+"/server-bootstrap", c.clientAccessInfo)
	if err != nil {
		return err
	}

	return bootstrap.Read(bytes.NewBuffer(content), &c.runtime.ControlRuntimeBootstrap)
}

func (c *Cluster) bootstrap(ctx context.Context) error {
	c.joining = true

	if c.runtime.HTTPBootstrap {
		return c.httpBootstrap()
	}

	if err := c.storageBootstrap(ctx); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) bootstrapStamp() string {
	return filepath.Join(c.config.DataDir, "db/joined-"+keyHash(c.config.Token))
}
