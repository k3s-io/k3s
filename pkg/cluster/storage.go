package cluster

import (
	"bytes"
	"context"
	"strings"

	"github.com/k3s-io/kine/pkg/client"
	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/sirupsen/logrus"
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

	storageClient, err := client.New(c.etcdConfig)
	if err != nil {
		return err
	}

	if err := storageClient.Create(ctx, storageKey(c.config.Token), data); err != nil {
		if err.Error() == "key exists" {
			logrus.Warnln("Bootstrap key exists. Please follow documentation updating a node after restore.")
			return nil
		} else if strings.Contains(err.Error(), "not supported for learner") {
			logrus.Debug("Skipping bootstrap data save on learner.")
			return nil
		}
		return err
	}

	return nil
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
