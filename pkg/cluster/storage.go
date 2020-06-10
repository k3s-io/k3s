package cluster

import (
	"bytes"
	"context"

	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/rancher/kine/pkg/client"
)

func (c *Cluster) save(ctx context.Context) error {
	buf := &bytes.Buffer{}
	if err := bootstrap.Write(buf, &c.runtime.ControlRuntimeBootstrap); err != nil {
		return err
	}

	data, err := encrypt(c.config.Token, buf.Bytes())
	if err != nil {
		return err
	}

	return c.storageClient.Create(ctx, storageKey(c.config.Token), data)
}

func (c *Cluster) storageBootstrap(ctx context.Context) error {
	if err := c.startStorage(ctx); err != nil {
		return err
	}

	storageClient, err := client.New(c.etcdConfig)
	if err != nil {
		return err
	}
	c.storageClient = storageClient

	value, err := storageClient.Get(ctx, storageKey(c.config.Token))
	if err == client.ErrNotFound {
		c.saveBootstrap = true
		return nil
	} else if err != nil {
		return err
	}

	data, err := decrypt(c.config.Token, value.Data)
	if err != nil {
		return err
	}

	return bootstrap.Read(bytes.NewBuffer(data), &c.runtime.ControlRuntimeBootstrap)
}
