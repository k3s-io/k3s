package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/sirupsen/logrus"
)

func (c *Cluster) Join(ctx context.Context) error {
	runJoin, err := c.shouldJoin()
	if err != nil {
		return err
	}
	c.runJoin = runJoin

	if runJoin {
		if err := c.join(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *Cluster) shouldJoin() (bool, error) {
	dqlite := c.dqliteEnabled()
	if dqlite {
		c.runtime.HTTPBootstrap = true
		if c.config.JoinURL == "" {
			return false, nil
		}
	}

	stamp := c.joinStamp()
	if _, err := os.Stat(stamp); err == nil {
		logrus.Info("Cluster bootstrap already complete")
		return false, nil
	}

	if dqlite && c.config.Token == "" {
		return false, fmt.Errorf("K3S_TOKEN is required to join a cluster")
	}

	return true, nil
}

func (c *Cluster) joined() error {
	if err := os.MkdirAll(filepath.Dir(c.joinStamp()), 0700); err != nil {
		return err
	}

	if _, err := os.Stat(c.joinStamp()); err == nil {
		return nil
	}

	f, err := os.Create(c.joinStamp())
	if err != nil {
		return err
	}

	return f.Close()
}

func (c *Cluster) httpJoin() error {
	token, err := clientaccess.NormalizeAndValidateTokenForUser(c.config.JoinURL, c.config.Token, "server")
	if err != nil {
		return err
	}

	info, err := clientaccess.ParseAndValidateToken(c.config.JoinURL, token)
	if err != nil {
		return err
	}
	c.clientAccessInfo = info

	content, err := clientaccess.Get("/v1-k3s/server-bootstrap", info)
	if err != nil {
		return err
	}

	return bootstrap.Read(bytes.NewBuffer(content), &c.runtime.ControlRuntimeBootstrap)
}

func (c *Cluster) join(ctx context.Context) error {
	c.joining = true

	if c.runtime.HTTPBootstrap {
		return c.httpJoin()
	}

	if err := c.storageJoin(ctx); err != nil {
		return err
	}

	return nil
}

func (c *Cluster) joinStamp() string {
	return filepath.Join(c.config.DataDir, "db/joined-"+keyHash(c.config.Token))
}
