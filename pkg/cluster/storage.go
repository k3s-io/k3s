package cluster

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/kine/pkg/client"
	"github.com/sirupsen/logrus"
)

// Save writes the current ControlRuntimeBootstrap data to the datastore. This contains a complete
// snapshot of the cluster's CA certs and keys, encryption passphrases, etc - encrypted with the join token.
// This is used when bootstrapping a cluster from a managed database or external etcd cluster.
// This is NOT used with embedded etcd, which bootstraps over HTTP.
func Save(ctx context.Context, config *config.Control, override bool) error {
	buf := &bytes.Buffer{}
	if err := bootstrap.ReadFromDisk(buf, &config.Runtime.ControlRuntimeBootstrap); err != nil {
		return err
	}
	token := config.Token
	if token == "" {
		tokenFromFile, err := readTokenFromFile(config.Runtime.ServerToken, config.Runtime.ServerCA, config.DataDir)
		if err != nil {
			return err
		}
		token = tokenFromFile
	}
	normalizedToken, err := normalizeToken(token)
	if err != nil {
		return err
	}

	data, err := encrypt(normalizedToken, buf.Bytes())
	if err != nil {
		return err
	}

	storageClient, err := client.New(config.Runtime.EtcdConfig)
	if err != nil {
		return err
	}
	defer storageClient.Close()

	if _, _, err = getBootstrapKeyFromStorage(ctx, storageClient, normalizedToken, token); err != nil {
		return err
	}

	if err := storageClient.Create(ctx, storageKey(normalizedToken), data); err != nil {
		if err.Error() == "key exists" {
			logrus.Warn("bootstrap key already exists")
			if override {
				bsd, err := bootstrapKeyData(ctx, storageClient)
				if err != nil {
					return err
				}
				return storageClient.Update(ctx, storageKey(normalizedToken), bsd.Modified, data)
			}
			return nil
		} else if strings.Contains(err.Error(), "not supported for learner") {
			logrus.Debug("skipping bootstrap data save on learner")
			return nil
		}
		return err
	}

	return nil
}

// bootstrapKeyData lists keys stored in the datastore with the prefix "/bootstrap", and
// will return the first such key. It will return an error if not exactly one key is found.
func bootstrapKeyData(ctx context.Context, storageClient client.Client) (*client.Value, error) {
	bootstrapList, err := storageClient.List(ctx, "/bootstrap", 0)
	if err != nil {
		return nil, err
	}
	if len(bootstrapList) == 0 {
		return nil, errors.New("no bootstrap data found")
	}
	if len(bootstrapList) > 1 {
		return nil, errors.New("found multiple bootstrap keys in storage")
	}
	return &bootstrapList[0], nil
}

// storageBootstrap loads data from the datastore into the ControlRuntimeBootstrap struct.
// The storage key and encryption passphrase are both derived from the join token.
// token is either passed.
func (c *Cluster) storageBootstrap(ctx context.Context) error {
	if err := c.startStorage(ctx); err != nil {
		return err
	}

	storageClient, err := client.New(c.config.Runtime.EtcdConfig)
	if err != nil {
		return err
	}
	defer storageClient.Close()

	token := c.config.Token
	if token == "" {
		tokenFromFile, err := readTokenFromFile(c.config.Runtime.ServerToken, c.config.Runtime.ServerCA, c.config.DataDir)
		if err != nil {
			return err
		}
		if tokenFromFile == "" {
			// at this point this is a fresh start in a non managed environment
			c.saveBootstrap = true
			return nil
		}
		token = tokenFromFile
	}
	normalizedToken, err := normalizeToken(token)
	if err != nil {
		return err
	}

	value, saveBootstrap, err := getBootstrapKeyFromStorage(ctx, storageClient, normalizedToken, token)
	c.saveBootstrap = saveBootstrap
	if err != nil {
		return err
	}
	if value == nil {
		return nil
	}

	data, err := decrypt(normalizedToken, value.Data)
	if err != nil {
		return err
	}

	return c.ReconcileBootstrapData(ctx, bytes.NewReader(data), &c.config.Runtime.ControlRuntimeBootstrap, false)
}

// getBootstrapKeyFromStorage will list all keys that has prefix /bootstrap and will check for key that is
// hashed with empty string and will check for any key that is hashed by different token than the one
// passed to it, it will return error if it finds a key that is hashed with different token and will return
// value if it finds the key hashed by passed token or empty string
func getBootstrapKeyFromStorage(ctx context.Context, storageClient client.Client, normalizedToken, oldToken string) (*client.Value, bool, error) {
	emptyStringKey := storageKey("")
	tokenKey := storageKey(normalizedToken)
	bootstrapList, err := storageClient.List(ctx, "/bootstrap", 0)
	if err != nil {
		return nil, false, err
	}
	if len(bootstrapList) == 0 {
		return nil, true, nil
	}
	if len(bootstrapList) > 1 {
		logrus.Warn("found multiple bootstrap keys in storage")
	}
	// check for empty string key and for old token format with k10 prefix
	if err := migrateOldTokens(ctx, bootstrapList, storageClient, emptyStringKey, tokenKey, normalizedToken, oldToken); err != nil {
		return nil, false, err
	}

	// getting the list of bootstrap again after migrating the empty key
	bootstrapList, err = storageClient.List(ctx, "/bootstrap", 0)
	if err != nil {
		return nil, false, err
	}
	for _, bootstrapKV := range bootstrapList {
		// ensure bootstrap is stored in the current token's key
		if string(bootstrapKV.Key) == tokenKey {
			return &bootstrapKV, false, nil
		}
	}

	return nil, false, errors.New("bootstrap data already found and encrypted with different token")
}

// readTokenFromFile will attempt to get the token from <data-dir>/token if it the file not found
// in case of fresh installation it will try to use the runtime serverToken saved in memory
// after stripping it from any additional information like the username or cahash, if the file
// found then it will still strip the token from any additional info
func readTokenFromFile(serverToken, certs, dataDir string) (string, error) {
	tokenFile := filepath.Join(dataDir, "token")

	b, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			token, err := clientaccess.FormatToken(serverToken, certs)
			if err != nil {
				return token, err
			}
			return token, nil
		}
		return "", err
	}

	// strip the token from any new line if its read from file
	return string(bytes.TrimRight(b, "\n")), nil
}

// normalizeToken will normalize the token read from file or passed as a cli flag
func normalizeToken(token string) (string, error) {
	_, password, ok := clientaccess.ParseUsernamePassword(token)
	if !ok {
		return password, errors.New("failed to normalize token; must be in format K10<CA-HASH>::<USERNAME>:<PASSWORD> or <PASSWORD>")
	}

	return password, nil
}

// migrateOldTokens will list all keys that has prefix /bootstrap and will check for key that is
// hashed with empty string and keys that is hashed with old token format before normalizing
// then migrate those and resave only with the normalized token
func migrateOldTokens(ctx context.Context, bootstrapList []client.Value, storageClient client.Client, emptyStringKey, tokenKey, token, oldToken string) error {
	oldTokenKey := storageKey(oldToken)

	for _, bootstrapKV := range bootstrapList {
		// checking for empty string bootstrap key
		if string(bootstrapKV.Key) == emptyStringKey {
			logrus.Warn("bootstrap data encrypted with empty string, deleting and resaving with token")
			if err := doMigrateToken(ctx, storageClient, bootstrapKV, "", emptyStringKey, token, tokenKey); err != nil {
				return err
			}
		} else if string(bootstrapKV.Key) == oldTokenKey && oldTokenKey != tokenKey {
			logrus.Warn("bootstrap data encrypted with old token format string, deleting and resaving with token")
			if err := doMigrateToken(ctx, storageClient, bootstrapKV, oldToken, oldTokenKey, token, tokenKey); err != nil {
				return err
			}
		}
	}

	return nil
}

func doMigrateToken(ctx context.Context, storageClient client.Client, keyValue client.Value, oldToken, oldTokenKey, newToken, newTokenKey string) error {
	// make sure that the process is non-destructive by decrypting/re-encrypting/storing the data before deleting the old key
	data, err := decrypt(oldToken, keyValue.Data)
	if err != nil {
		return err
	}

	encryptedData, err := encrypt(newToken, data)
	if err != nil {
		return err
	}

	// saving the new encrypted data with the right token key
	if err := storageClient.Create(ctx, newTokenKey, encryptedData); err != nil {
		if err.Error() == "key exists" {
			logrus.Warn("bootstrap key exists")
		} else if strings.Contains(err.Error(), "not supported for learner") {
			logrus.Debug("skipping bootstrap data save on learner")
			return nil
		} else {
			return err
		}
	}

	logrus.Infof("created bootstrap key %s", newTokenKey)
	// deleting the old key
	if err := storageClient.Delete(ctx, oldTokenKey, keyValue.Modified); err != nil {
		logrus.Warnf("failed to delete old bootstrap key %s", oldTokenKey)
	}

	return nil
}
