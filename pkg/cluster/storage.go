package cluster

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/k3s-io/kine/pkg/client"
	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/rancher/k3s/pkg/clientaccess"
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
	token := c.config.Token
	if token == "" {
		tokenFromFile, err := getTokenFromFile(c.runtime.ServerToken, c.config.DataDir)
		if err != nil {
			return err
		}
		token = tokenFromFile
	}
	data, err := encrypt(token, buf.Bytes())
	if err != nil {
		return err
	}

	storageClient, err := client.New(c.etcdConfig)
	if err != nil {
		return err
	}

	_, _, err = c.getBootstrapKeyFromStorage(ctx, storageClient, token)
	if err != nil {
		return err
	}

	if err := storageClient.Create(ctx, storageKey(token), data); err != nil {
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

	token := c.config.Token
	if token == "" {
		tokenFromFile, err := getTokenFromFile(c.runtime.ServerToken, c.config.DataDir)
		if err != nil {
			return err
		}
		token = tokenFromFile
	}

	value, emptyKey, err := c.getBootstrapKeyFromStorage(ctx, storageClient, token)
	if err != nil {
		return err
	}
	if value == nil {
		return nil
	}
	if emptyKey {
		token = ""
	}
	data, err := decrypt(token, value.Data)
	if err != nil {
		return err
	}

	return bootstrap.Read(bytes.NewBuffer(data), &c.runtime.ControlRuntimeBootstrap)
}

// getBootstrapKeyFromStorage will list all keys that has prefix /bootstrap and will check for key that is
// hashed with empty string and will check for any key that is hashed by different token than the one
// passed to it, it will return error if it finds a key that is hashed with different token and will return
// value if it finds the key hashed by passed token or empty string
func (c *Cluster) getBootstrapKeyFromStorage(ctx context.Context, storageClient client.Client, token string) (*client.Value, bool, error) {
	emptyStringKey := storageKey("")
	tokenKey := storageKey(token)

	oldBootstrapValues, err := storageClient.List(ctx, "/bootstrap", 0)
	if err != nil {
		if err != client.ErrNotFound {
			c.saveBootstrap = true
			return nil, false, nil
		}
		return nil, false, err
	}
	for _, v := range oldBootstrapValues {
		switch string(v.Key) {
		case emptyStringKey:
			logrus.Warnf("Bootstrap data already found and encrypted with empty string")
			c.saveBootstrap = true
			return &v, true, nil
		case tokenKey:
			return &v, false, nil
		default:
			return nil, false, fmt.Errorf("Bootstrap data already found and encrypted with different token")
		}
	}
	return nil, false, nil
}

// getToken will attempt to get the token from <data-dir>/token if it the file not found
// in case of fresh installation it will try to use the runtime serverToken saved in memory
// after stripping it from any additional information like the username or cahash, if the file
// found then it will still strip the token from any additional info
func getTokenFromFile(serverToken, dataDir string) (string, error) {
	tokenFile := filepath.Join(dataDir, "token")
	tokenByte, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			// attempt to use the serverToken
			parts := strings.Split(serverToken, ":")
			if len(parts) > 0 {
				return strings.TrimSuffix(parts[1], "\n"), nil
			}
			return "", fmt.Errorf("server token is invalid")
		}
		return "", err
	}
	info, err := clientaccess.ParseToken(strings.TrimSuffix(string(tokenByte), "\n"))
	if err != nil {
		return "", err
	}
	return info.Password, nil
}
