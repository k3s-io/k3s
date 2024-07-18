package cluster

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/kine/pkg/client"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"k8s.io/apimachinery/pkg/util/wait"
)

// maxBootstrapWaitAttempts is the number of iterations to wait for another node to populate an empty bootstrap key.
// After this many attempts, the lock is deleted and the counter reset.
const maxBootstrapWaitAttempts = 5

func RotateBootstrapToken(ctx context.Context, config *config.Control, oldToken string) error {
	token, err := util.ReadTokenFromFile(config.Runtime.ServerToken, config.Runtime.ServerCA, config.DataDir)
	if err != nil {
		return err
	}

	normalizedToken, err := util.NormalizeToken(token)
	if err != nil {
		return err
	}

	storageClient, err := client.New(config.Runtime.EtcdConfig)
	if err != nil {
		return err
	}
	defer storageClient.Close()

	tokenKey := storageKey(normalizedToken)

	var bootstrapList []client.Value
	if err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		bootstrapList, err = storageClient.List(ctx, "/bootstrap", 0)
		if err != nil {
			return false, err
		}
		return true, nil
	}); err != nil {
		return err
	}

	normalizedOldToken, err := util.NormalizeToken(oldToken)
	if err != nil {
		return err
	}
	// reuse the existing migration function to reencrypt bootstrap data with new token
	if err := migrateTokens(ctx, bootstrapList, storageClient, "", tokenKey, normalizedToken, normalizedOldToken); err != nil {
		return err
	}

	return nil
}

// Save writes the current ControlRuntimeBootstrap data to the datastore. This contains a complete
// snapshot of the cluster's CA certs and keys, encryption passphrases, etc - encrypted with the join token.
// This is used when bootstrapping a cluster from a managed database or external etcd cluster.
// This is NOT used with embedded etcd, which bootstraps over HTTP.
func Save(ctx context.Context, config *config.Control, override bool) error {
	logrus.Info("Saving cluster bootstrap data to datastore")
	buf := &bytes.Buffer{}
	if err := bootstrap.ReadFromDisk(buf, &config.Runtime.ControlRuntimeBootstrap); err != nil {
		return err
	}
	token := config.Token
	if token == "" {
		tokenFromFile, err := util.ReadTokenFromFile(config.Runtime.ServerToken, config.Runtime.ServerCA, config.DataDir)
		if err != nil {
			return err
		}
		token = tokenFromFile
	}
	normalizedToken, err := util.NormalizeToken(token)
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

	currentKey, _, err := getBootstrapKeyFromStorage(ctx, storageClient, normalizedToken, token)
	if err != nil {
		return err
	}

	// If there's an empty bootstrap key, then we've locked it and can override.
	if currentKey != nil && len(currentKey.Data) == 0 {
		logrus.Info("Bootstrap key lock is held")
		override = true
	}

	if err := storageClient.Create(ctx, storageKey(normalizedToken), data); err != nil {
		if err.Error() == "key exists" {
			if override {
				bsd, err := bootstrapKeyData(ctx, storageClient)
				if err != nil {
					return err
				}
				return storageClient.Update(ctx, storageKey(normalizedToken), bsd.Modified, data)
			}
			logrus.Warn("Bootstrap key already exists")
			return nil
		} else if errors.Is(err, rpctypes.ErrGPRCNotSupportedForLearner) {
			logrus.Debug("Skipping bootstrap data save on learner")
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

// storageBootstrap loads data from the datastore's bootstrap key into the
// ControlRuntimeBootstrap struct. The storage key and encryption passphrase are both derived
// from the join token. If no bootstrap key exists, indicating that data needs to be written
// back to the datastore, this function will set c.saveBootstrap to true and create an empty
// bootstrap key as a lock. This function will not return successfully until either the
// bootstrap key has been locked, or data is read into the struct.
func (c *Cluster) storageBootstrap(ctx context.Context) error {
	if c.config.KineTLS {
		bootstrapCtx, cancel := context.WithCancel(ctx)
		defer func() {
			time.Sleep(time.Second)
			cancel()
		}()

		logrus.Info("Starting temporary kine to reconcile with datastore")
		if err := c.startStorage(bootstrapCtx, true); err != nil {
			return err
		}
	} else {
		if err := c.startStorage(ctx, true); err != nil {
			return err
		}
	}

	storageClient, err := client.New(c.config.Runtime.EtcdConfig)
	if err != nil {
		return err
	}
	defer storageClient.Close()

	token := c.config.Token
	if token == "" {
		tokenFromFile, err := util.ReadTokenFromFile(c.config.Runtime.ServerToken, c.config.Runtime.ServerCA, c.config.DataDir)
		if err != nil {
			return err
		}
		if tokenFromFile == "" {
			// No token on disk or from CLI, but we don't know if there's data in the datastore.
			// Return here and generate new CA certs and tokens. Note that startup will fail
			// later when saving to the datastore if there's already a bootstrap key - but
			// that's AFTER generating CA certs and tokens. If the config is updated to set the
			// matching key, further startups will still be blocked pending cleanup of the
			// "newer" files as per the bootstrap reconciliation code.
			c.saveBootstrap = true
			return nil
		}
		token = tokenFromFile
	}
	normalizedToken, err := util.NormalizeToken(token)
	if err != nil {
		return err
	}

	attempts := 0
	tokenKey := storageKey(normalizedToken)
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		attempts++
		value, saveBootstrap, err := getBootstrapKeyFromStorage(ctx, storageClient, normalizedToken, token)
		c.saveBootstrap = saveBootstrap
		if err != nil {
			return false, err
		}

		if value == nil {
			// No bootstrap keys found in the datastore - create an empty bootstrap key as a lock to
			// ensure that no other node races us to populate it.  If we fail to create the key, then
			// some other node beat us to it and we should just wait for them to finish.
			if err := storageClient.Create(ctx, tokenKey, []byte{}); err != nil {
				if err.Error() == "key exists" {
					logrus.Info("Bootstrap key already locked - waiting for data to be populated by another server")
					return false, nil
				}
				return false, err
			}
			logrus.Info("Bootstrap key locked for initial create")
			return true, nil
		}

		if len(value.Data) == 0 {
			// Empty (locked) bootstrap key found - check to see if we should continue waiting, or
			// delete it and attempt to retake the lock on the next iteration (assuming that the
			// other node failed while holding the lock).
			if attempts >= maxBootstrapWaitAttempts {
				logrus.Info("Bootstrap key lock timed out - deleting lock and retrying")
				attempts = 0
				if err := storageClient.Delete(ctx, tokenKey, value.Modified); err != nil {
					return false, err
				}
			} else {
				logrus.Infof("Bootstrap key is locked - waiting for data to be populated by another server")
			}
			return false, nil
		}

		data, err := decrypt(normalizedToken, value.Data)
		if err != nil {
			return false, err
		}

		return true, c.ReconcileBootstrapData(ctx, bytes.NewReader(data), &c.config.Runtime.ControlRuntimeBootstrap, false)
	})
}

// getBootstrapKeyFromStorage will list all keys that has prefix /bootstrap and will check for key that is
// hashed with empty string and will check for any key that is hashed by different token than the one
// passed to it, it will return error if it finds a key that is hashed with different token and will return
// value if it finds the key hashed by passed token or empty string.
// Upon receiving a "not supported for learner" error from etcd, this function will retry until the context is cancelled.
func getBootstrapKeyFromStorage(ctx context.Context, storageClient client.Client, normalizedToken, oldToken string) (*client.Value, bool, error) {
	emptyStringKey := storageKey("")
	tokenKey := storageKey(normalizedToken)

	var bootstrapList []client.Value
	var err error

	if err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		bootstrapList, err = storageClient.List(ctx, "/bootstrap", 0)
		if err != nil {
			if errors.Is(err, rpctypes.ErrGPRCNotSupportedForLearner) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, false, err
	}

	if len(bootstrapList) == 0 {
		return nil, true, nil
	}
	if len(bootstrapList) > 1 {
		logrus.Warn("found multiple bootstrap keys in storage")
	}
	// check for empty string key and for old token format with k10 prefix
	if err := migrateTokens(ctx, bootstrapList, storageClient, emptyStringKey, tokenKey, normalizedToken, oldToken); err != nil {
		return nil, false, err
	}

	// getting the list of bootstrap again after migrating the empty key
	bootstrapList, err = storageClient.List(ctx, "/bootstrap", 0)
	if err != nil {
		return nil, false, err
	}
	for _, bootstrapKV := range bootstrapList {
		// ensure bootstrap is stored in the current token's key
		logrus.Debugf("checking bootstrap key %s against %s", string(bootstrapKV.Key), tokenKey)
		if string(bootstrapKV.Key) == tokenKey {
			return &bootstrapKV, false, nil
		}
	}

	return nil, false, errors.New("bootstrap data already found and encrypted with different token")
}

// migrateTokens will list all keys that has prefix /bootstrap and will check for key that is
// hashed with empty string and keys that is hashed with old token format before normalizing
// then migrate those and resave only with the normalized token
func migrateTokens(ctx context.Context, bootstrapList []client.Value, storageClient client.Client, emptyStringKey, tokenKey, token, oldToken string) error {
	oldTokenKey := storageKey(oldToken)

	for _, bootstrapKV := range bootstrapList {
		// checking for empty string bootstrap key
		logrus.Debug("Comparing ", string(bootstrapKV.Key), " to ", oldTokenKey)
		if string(bootstrapKV.Key) == emptyStringKey {
			logrus.Warn("Bootstrap data encrypted with empty string, deleting and resaving with token")
			if err := doMigrateToken(ctx, storageClient, bootstrapKV, "", emptyStringKey, token, tokenKey); err != nil {
				return err
			}
		} else if string(bootstrapKV.Key) == oldTokenKey && oldTokenKey != tokenKey {
			if emptyStringKey != "" {
				logrus.Warn("bootstrap data encrypted with old token format string, deleting and resaving with token")
			}
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
		} else if errors.Is(err, rpctypes.ErrGPRCNotSupportedForLearner) {
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
