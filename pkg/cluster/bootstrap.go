package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
<<<<<<< HEAD
||||||| parent of a6fe2c0bc5... Resolve restore bootstrap (#4704)
	"strings"
	"time"
=======
	"strconv"
	"strings"
	"time"
>>>>>>> a6fe2c0bc5... Resolve restore bootstrap (#4704)

<<<<<<< HEAD
||||||| parent of a6fe2c0bc5... Resolve restore bootstrap (#4704)
	"github.com/k3s-io/kine/pkg/client"
	"github.com/k3s-io/kine/pkg/endpoint"
=======
	"github.com/k3s-io/kine/pkg/client"
	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/otiai10/copy"
>>>>>>> a6fe2c0bc5... Resolve restore bootstrap (#4704)
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

	if shouldBootstrap {
		if err := c.bootstrap(ctx); err != nil {
			return err
		}
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

	// No errors and no bootstrap stamp, need to bootstrap.
	return true, false, nil
}

// isDirEmpty checks to see if the given directory
// is empty.
func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}

	// No errors and no bootstrap stamp, need to bootstrap.
	return true, nil
}

// bootstrapped touches a file to indicate that bootstrap has been completed.
func (c *Cluster) bootstrapped() error {
	stamp := c.bootstrapStamp()
	if err := os.MkdirAll(filepath.Dir(stamp), 0700); err != nil {
		return err
	}

	// return if file already exists
	if _, err := os.Stat(stamp); err == nil {
		return nil
	}

	// otherwise try to create it
	f, err := os.Create(stamp)
	if err != nil {
		return err
	}

	files := make(bootstrap.PathsDataformat)
	if err := json.NewDecoder(buf).Decode(&files); err != nil {
		// This will fail if data is being pulled from old an cluster since
		// older clusters used a map[string][]byte for the data structure.
		// Therefore, we need to perform a migration to the newer bootstrap
		// format; bootstrap.BootstrapFile.
		buf.Seek(0, 0)

		if err := migrateBootstrapData(ctx, buf, files); err != nil {
			return err
		}
	}
	buf.Seek(0, 0)

	type update struct {
		db, disk, conflict bool
	}

	var updateDisk bool

	results := make(map[string]update)
	for pathKey, fileData := range files {
		path, ok := paths[pathKey]
		if !ok {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Warn(path + " doesn't exist. continuing...")
				updateDisk = true
				continue
			}
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

	if c.config.ClusterReset {
		serverTLSDir := filepath.Join(c.config.DataDir, "tls")
		tlsBackupDir := filepath.Join(c.config.DataDir, "tls-"+strconv.Itoa(int(time.Now().Unix())))

		logrus.Infof("Cluster reset: backing up certificates directory to " + tlsBackupDir)

		if _, err := os.Stat(serverTLSDir); err != nil {
			return err
		}
		if err := copy.Copy(serverTLSDir, tlsBackupDir); err != nil {
			return err
		}
	}

	for path, res := range results {
		switch {
		case res.disk:
			updateDisk = true
			logrus.Warn("datastore newer than " + path)
		case res.db:
			if c.config.ClusterReset {
				logrus.Infof("Cluster reset: replacing file on disk: " + path)
				updateDisk = true
				continue
			}
			logrus.Fatal(path + " newer than datastore and could cause cluster outage. Remove the file from disk and restart to be recreated from datastore.")
		case res.conflict:
			logrus.Warnf("datastore / disk conflict: %s newer than in the datastore", path)
		}
	}

	if updateDisk {
		logrus.Warn("updating bootstrap data on disk from datastore")
		return bootstrap.WriteToDiskFromStorage(buf, crb)
	}

	return nil
}

// httpBootstrap retrieves bootstrap data (certs and keys, etc) from the remote server via HTTP
// and loads it into the ControlRuntimeBootstrap struct. Unlike the storage bootstrap path,
// this data does not need to be decrypted since it is generated on-demand by an existing server.
func (c *Cluster) httpBootstrap() error {
	content, err := clientaccess.Get("/v1-"+version.Program+"/server-bootstrap", c.clientAccessInfo)
	if err != nil {
		return err
	}

	return bootstrap.Read(bytes.NewBuffer(content), &c.runtime.ControlRuntimeBootstrap)
}

// bootstrap performs cluster bootstrapping, either via HTTP (for managed databases) or direct load from datastore.
func (c *Cluster) bootstrap(ctx context.Context) error {
	c.joining = true

	// bootstrap managed database via HTTPS
	if c.runtime.HTTPBootstrap {
		// Assuming we should just compare on managed databases
		if err := c.compareConfig(); err != nil {
			return err
		}
		return c.httpBootstrap()
	}

	// Bootstrap directly from datastore
	return c.storageBootstrap(ctx)
}

// bootstrapStamp returns the path to a file in datadir/db that is used to record
// that a cluster has been joined. The filename is based on a portion of the sha256 hash of the token.
// We hash the token value exactly as it is provided by the user, NOT the normalized version.
func (c *Cluster) bootstrapStamp() string {
	return filepath.Join(c.config.DataDir, "db/joined-"+keyHash(c.config.Token))
}

// Snapshot is a proxy method to call the snapshot method on the managedb
// interface for etcd clusters.
func (c *Cluster) Snapshot(ctx context.Context, config *config.Control) error {
	if c.managedDB == nil {
		return errors.New("unable to perform etcd snapshot on non-etcd system")
	}
	return c.managedDB.Snapshot(ctx, config)
}

// compareConfig verifies that the config of the joining control plane node coincides with the cluster's config
func (c *Cluster) compareConfig() error {
	agentClientAccessInfo, err := clientaccess.ParseAndValidateTokenForUser(c.config.JoinURL, c.config.Token, "node")
	if err != nil {
		return err
	}
	serverConfig, err := clientaccess.Get("/v1-"+version.Program+"/config", agentClientAccessInfo)
	if err != nil {
		return err
	}
	clusterControl := &config.Control{}
	if err := json.Unmarshal(serverConfig, clusterControl); err != nil {
		return err
	}

	// We are saving IPs of ClusterIPRanges and ServiceIPRanges in 4-bytes representation but json decodes in 16-byte
	c.config.CriticalControlArgs.ClusterIPRange.IP.To16()
	c.config.CriticalControlArgs.ServiceIPRange.IP.To16()

	if !reflect.DeepEqual(clusterControl.CriticalControlArgs, c.config.CriticalControlArgs) {
		logrus.Debugf("This is the server CriticalControlArgs: %#v", clusterControl.CriticalControlArgs)
		logrus.Debugf("This is the local CriticalControlArgs: %#v", c.config.CriticalControlArgs)
		return errors.New("unable to join cluster due to critical configuration value mismatch")
	}
	return nil
}
