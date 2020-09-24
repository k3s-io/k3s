package cluster

import (
	"bytes"
	"context"

	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/rancher/kine/pkg/client"
)

// save writes the current ControlRuntimeBootstrap data to the datastore. This contains a complete
// snapshot of the cluster's CA certs and keys, encryption passphrases, etc - encrypted with the join token.
// This is used when bootstrapping a cluster from a managed database or external etcd cluster.
// This is NOT used with embedded etcd, which bootstraps over HTTP.
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

// storageBootstrap loads data from the datastore into the ControlRuntimeBootstrap struct.
// The storage key and encryption passphrase are both derived from the join token.
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
