package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k3s-io/kine/pkg/client"
	"github.com/rancher/k3s/pkg/bootstrap"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

// Bootstrap attempts to load a managed database driver, if one has been initialized or should be created/joined.
// It then checks to see if the cluster needs to load bootstrap data, and if so, loads data into the
// ControlRuntimeBoostrap struct, either via HTTP or from the datastore.
func (c *Cluster) Bootstrap(ctx context.Context) error {
	if err := c.assignManagedDriver(ctx); err != nil {
		return err
	}

	shouldBootstrap, err := c.shouldBootstrapLoad(ctx)
	if err != nil {
		return err
	}
	c.shouldBootstrap = shouldBootstrap

	if c.shouldBootstrap {
		return c.bootstrap(ctx)
	}

	return nil
}

// shouldBootstrapLoad returns true if we need to load ControlRuntimeBootstrap data again.
// This is controlled by a stamp file on disk that records successful bootstrap using a hash of the join token.
func (c *Cluster) shouldBootstrapLoad(ctx context.Context) (bool, error) {
	// Non-nil managedDB indicates that the database is either initialized, initializing, or joining
	if c.managedDB != nil {
		c.runtime.HTTPBootstrap = true

		isInitialized, err := c.managedDB.IsInitialized(ctx, c.config)
		if err != nil {
			return false, err
		}

		if isInitialized {
			// If the database is initialized we skip bootstrapping; if the user wants to rejoin a
			// cluster they need to delete the database.
			logrus.Infof("Managed %s cluster bootstrap already complete and initialized", c.managedDB.EndpointName())
			// This is a workaround for an issue that can be caused by terminating the cluster bootstrap before
			// etcd is promoted from learner. Odds are we won't need this info, and we don't want to fail startup
			// due to failure to retrieve it as this will break cold cluster restart, so we ignore any errors.
			if c.config.JoinURL != "" && c.config.Token != "" {
				c.clientAccessInfo, _ = clientaccess.ParseAndValidateTokenForUser(c.config.JoinURL, c.config.Token, "server")
			}
			return false, nil
		} else if c.config.JoinURL == "" {
			// Not initialized, not joining - must be initializing (cluster-init)
			logrus.Infof("Managed %s cluster initializing", c.managedDB.EndpointName())
			return false, nil
		} else {
			// Not initialized, but have a Join URL - fail if there's no token; if there is then validate it.
			if c.config.Token == "" {
				return false, errors.New(version.ProgramUpper + "_TOKEN is required to join a cluster")
			}

			// Fail if the token isn't syntactically valid, or if the CA hash on the remote server doesn't match
			// the hash in the token. The password isn't actually checked until later when actually bootstrapping.
			info, err := clientaccess.ParseAndValidateTokenForUser(c.config.JoinURL, c.config.Token, "server")
			if err != nil {
				return false, err
			}

			logrus.Infof("Managed %s cluster not yet initialized", c.managedDB.EndpointName())
			c.clientAccessInfo = info
		}
	}

	// Check the stamp file to see if we have successfully bootstrapped using this token.
	// NOTE: The fact that we use a hash of the token to generate the stamp
	//       means that it is unsafe to use the same token for multiple clusters.
	// stamp := c.bootstrapStamp()
	// if _, err := os.Stat(stamp); err == nil {
	// 	logrus.Info("Cluster bootstrap already complete")
	// 	return false, nil
	// }

	// No errors and no bootstrap stamp, need to bootstrap.
	return true, nil
}

// isDirEmpty checks to see if the given directory
// is empty.
func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}

// certDirsExist checks to see if the directories
// that contain the needed certificates exist.
func (c *Cluster) certDirsExist() error {
	bootstrapDirs := []string{
		"tls",
		"tls/etcd",
	}

	const (
		missingDir = "missing %s directory from ${data-dir}"
		emptyDir   = "%s directory is empty"
	)

	for _, dir := range bootstrapDirs {
		d := filepath.Join(c.config.DataDir, dir)
		if _, err := os.Stat(d); os.IsNotExist(err) {
			errMsg := fmt.Sprintf(missingDir, d)
			logrus.Debug(errMsg)
			return errors.New(errMsg)
		}

		ok, err := isDirEmpty(d)
		if err != nil {
			return err
		}

		if ok {
			errMsg := fmt.Sprintf(emptyDir, d)
			logrus.Debug(errMsg)
			return errors.New(errMsg)
		}
	}

	return nil
}

// migrateBootstrapData migrates bootstrap data from the old format to the new format.
func (c *Cluster) migrateBootstrapData(ctx context.Context, sc client.Client, data []byte, files bootstrap.PathsDataformat, normalizedToken string, rev int64) error {
	logrus.Info("Migrating bootstrap data to new format")

	var oldBootstrapData map[string][]byte
	if err := json.NewDecoder(bytes.NewBuffer(data)).Decode(&oldBootstrapData); err != nil {
		// if this errors here, we can assume that the error being thrown
		// is not related to needing to perform a migration.
		return err
	}

	// iterate through the old bootstrap data structure
	// and copy into the new bootstrap data structure
	for k, v := range oldBootstrapData {
		files[k] = bootstrap.File{
			Content: v,
		}
	}

	return sc.Update(ctx, storageKey(normalizedToken), rev, data)
}

const systemTimeSkew = int64(3)

// ReconcileBootstrapData is called before any data is saved to the
// datastore or locally. It checks to see if the contents of the
// bootstrap data in the datastore is newer than on disk or different
//  and dependingon where the difference is, the newer data is written
// to the older.
func (c *Cluster) ReconcileBootstrapData(ctx context.Context, crb *config.ControlRuntimeBootstrap) error {
	logrus.Info("Reconciling bootstrap data between datastore and disk")

	storageClient, err := client.New(c.etcdConfig)
	if err != nil {
		return err
	}

	token := c.config.Token
	if token == "" {
		tokenFromFile, err := readTokenFromFile(c.runtime.ServerToken, c.runtime.ServerCA, c.config.DataDir)
		if err != nil {
			return err
		}
		if tokenFromFile == "" {
			// at this point this is a fresh start in a non-managed environment
			c.saveBootstrap = true
			return nil
		}
		token = tokenFromFile
	}
	normalizedToken, err := normalizeToken(token)
	if err != nil {
		return err
	}

	var value *client.Value

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		value, err = c.getBootstrapKeyFromStorage(ctx, storageClient, normalizedToken, token)
		if err != nil {
			if strings.Contains(err.Error(), "not supported for learner") { // create error value
				continue
			}
			return err
		}
		if value == nil {
			return nil
		}

		break
	}

	data, err := decrypt(normalizedToken, value.Data)
	if err != nil {
		return err
	}

	if err := c.certDirsExist(); err != nil {
		logrus.Warn(err.Error())
		return bootstrap.WriteToDiskFromStorage(bytes.NewBuffer(data), crb)
	}

	paths, err := bootstrap.ObjToMap(crb)
	if err != nil {
		return err
	}

	files := make(bootstrap.PathsDataformat)
	if err := json.NewDecoder(bytes.NewBuffer(data)).Decode(&files); err != nil {
		// This will fail if data is being pulled from old an cluster since
		// older clusters used a map[string][]byte for the data structure.
		// Therefore, we need to perform a migration to the newer bootstrap
		// format; bootstrap.BootstrapFile.
		if err := c.migrateBootstrapData(ctx, storageClient, data, files, normalizedToken, value.Modified); err != nil {
			return err
		}
	}

	type update struct {
		db, disk, conflict bool
	}

	var updateDatastore, updateDisk bool

	results := make(map[string]update)

	for pathKey, fileData := range files {
		path, ok := paths[pathKey]
		if !ok {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		fData, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		if !bytes.Equal(fileData.Content, fData) {
			info, err := os.Stat(path)
			if err != nil {
				return err
			}

			switch {
			case info.ModTime().Unix()-files[pathKey].Timestamp.Unix() >= systemTimeSkew:
				if _, ok := results[path]; !ok {
					results[path] = update{
						db: true,
					}
				}

				for pk := range files {
					p, ok := paths[pk]
					if !ok {
						continue
					}

					if filepath.Base(p) == info.Name() {
						continue
					}

					i, err := os.Stat(p)
					if err != nil {
						return err
					}

					if i.ModTime().Unix()-files[pk].Timestamp.Unix() >= systemTimeSkew {
						if _, ok := results[path]; !ok {
							results[path] = update{
								conflict: true,
							}
						}
					}
				}
			case info.ModTime().Unix()-files[pathKey].Timestamp.Unix() <= systemTimeSkew:
				if _, ok := results[info.Name()]; !ok {
					results[path] = update{
						disk: true,
					}
				}

				for pk := range files {
					p, ok := paths[pk]
					if !ok {
						continue
					}

					if filepath.Base(p) == info.Name() {
						continue
					}

					i, err := os.Stat(p)
					if err != nil {
						return err
					}

					if i.ModTime().Unix()-files[pk].Timestamp.Unix() <= systemTimeSkew {
						if _, ok := results[path]; !ok {
							results[path] = update{
								conflict: true,
							}
						}
					}
				}
			default:
				if _, ok := results[path]; ok {
					results[path] = update{}
				}
			}
		}
	}

	for path, res := range results {
		if res.db {
			updateDatastore = true
			logrus.Warn(path + " newer than datastore")
		} else if res.disk {
			updateDisk = true
			logrus.Warn("datastore newer than " + path)
		} else if res.conflict {
			logrus.Warnf("datastore / disk conflict: %s newer than in the datastore", path)
		}
	}

	switch {
	case updateDatastore:
		logrus.Warn("updating bootstrap data in datastore from disk")
		return c.save(ctx, true)
	case updateDisk:
		logrus.Warn("updating bootstrap data on disk from datastore")
		return bootstrap.WriteToDiskFromStorage(bytes.NewBuffer(data), crb)
	default:
		// on disk certificates match timestamps in storage. noop.
	}

	return nil
}

// httpBootstrap retrieves bootstrap data (certs and keys, etc) from the remote server via HTTP
// and loads it into the ControlRuntimeBootstrap struct. Unlike the storage bootstrap path,
// this data does not need to be decrypted since it is generated on-demand by an existing server.
func (c *Cluster) httpBootstrap(ctx context.Context) error {
	content, err := c.clientAccessInfo.Get("/v1-" + version.Program + "/server-bootstrap")
	if err != nil {
		return err
	}

	return bootstrap.WriteToDiskFromStorage(bytes.NewBuffer(content), &c.runtime.ControlRuntimeBootstrap)
}

// bootstrap performs cluster bootstrapping, either via HTTP (for managed databases) or direct load from datastore.
func (c *Cluster) bootstrap(ctx context.Context) error {
	c.joining = true

	// bootstrap managed database via HTTPS
	if c.runtime.HTTPBootstrap {
		return c.httpBootstrap(ctx)
	}

	// Bootstrap directly from datastore
	return c.storageBootstrap(ctx)
}

// Snapshot is a proxy method to call the snapshot method on the managedb
// interface for etcd clusters.
func (c *Cluster) Snapshot(ctx context.Context, config *config.Control) error {
	if c.managedDB == nil {
		return errors.New("unable to perform etcd snapshot on non-etcd system")
	}
	return c.managedDB.Snapshot(ctx, config)
}
