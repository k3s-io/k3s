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

	// shouldBootstrap, err := c.shouldBootstrapLoad(ctx)
	// if err != nil {
	// 	return err
	// }
	// c.shouldBootstrap = shouldBootstrap

	return c.bootstrap(ctx)
}

// shouldBootstrapLoad returns true if we need to load ControlRuntimeBootstrap data again.
// This is controlled by a stamp file on disk that records successful bootstrap using a hash of the join token.
func (c *Cluster) bootstrapSetup(ctx context.Context) error {
	// Non-nil managedDB indicates that the database is either initialized, initializing, or joining
	if c.managedDB != nil {
		c.runtime.HTTPBootstrap = true

		isInitialized, err := c.managedDB.IsInitialized(ctx, c.config)
		if err != nil {
			return err
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
			return nil
		}
		if c.config.JoinURL == "" {
			// Not initialized, not joining - must be initializing (cluster-init)
			logrus.Infof("Managed %s cluster initializing", c.managedDB.EndpointName())
			return nil
		} else {
			// Not initialized, but have a Join URL - fail if there's no token; if there is then validate it.
			if c.config.Token == "" {
				return errors.New(version.ProgramUpper + "_TOKEN is required to join a cluster")
			}

			// Fail if the token isn't syntactically valid, or if the CA hash on the remote server doesn't match
			// the hash in the token. The password isn't actually checked until later when actually bootstrapping.
			info, err := clientaccess.ParseAndValidateTokenForUser(c.config.JoinURL, c.config.Token, "server")
			if err != nil {
				return err
			}

			logrus.Infof("Managed %s cluster not yet initialized", c.managedDB.EndpointName())
			c.clientAccessInfo = info
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
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			errMsg := fmt.Sprintf(missingDir, dir)
			logrus.Debug(errMsg)
			return errors.New(errMsg)
		}

		ok, err := isDirEmpty(filepath.Join(c.config.DataDir, dir))
		if err != nil {
			return err
		}

		if ok {
			errMsg := fmt.Sprintf(emptyDir, dir)
			logrus.Debug(errMsg)
			return errors.New(errMsg)
		}
	}

	return nil
}

// ReconcileStorage is called before any data is saved to the datastore
// or locally. It checks to see if the contents of the bootstrap data
// in the datastore is newer than on disk or different and depending
// on where the difference is, the newer data is written to the older.
func (c *Cluster) ReconcileStorage(ctx context.Context, r io.Reader, crb *config.ControlRuntimeBootstrap) error {
	if err := c.certDirsExist(); err != nil {
		logrus.Warn(err.Error())
		return bootstrap.WriteToDiskFromStorage(r, crb)
	}

	paths, err := bootstrap.ObjToMap(crb)
	if err != nil {
		return err
	}

	files := make(map[string]bootstrap.BootstrapFile)
	if err := json.NewDecoder(r).Decode(&files); err != nil {
		return err
	}

	for pathKey, data := range files {
		path, ok := paths[pathKey]
		if !ok {
			continue
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		fd, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		logrus.Warnf("XXX - checking %s", path)
		if !bytes.Equal(data.Content, fd) {
			logrus.Warnf("%s is out of sync with datastore", path)

			return c.save(ctx, true)
		}

		info, err := os.Stat(path)
		if err != nil {
			return err
		}

		// TODO(): allow for a few seconds between the 2 so get
		// seconds

		switch {
		//case (info.ModTime().Unix() - files[pathKey].Timestamp.Unix()) <= 4 || (info.ModTime().Unix() - files[pathKey].Timestamp.Unix()) >= -4:
		case info.ModTime().Unix() > files[pathKey].Timestamp.Unix():
			logrus.Warn("on disk file newer than database")
			return c.save(ctx, true)
		case info.ModTime().Unix() < files[pathKey].Timestamp.Unix():
			logrus.Warn("database newer than on disk file")
			return bootstrap.WriteToDiskFromStorage(r, crb)
		default:
			// on disk certificates match what is in storage, noop
		}
	}

	return nil
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

	return f.Close()
}

// httpBootstrap retrieves bootstrap data (certs and keys, etc) from the remote server via HTTP
// and loads it into the ControlRuntimeBootstrap struct. Unlike the storage bootstrap path,
// this data does not need to be decrypted since it is generated on-demand by an existing server.
func (c *Cluster) httpBootstrap(ctx context.Context) error {
	logrus.Warn("XXX - http boostrapping")
	content, err := c.clientAccessInfo.Get("/v1-" + version.Program + "/server-bootstrap")
	if err != nil {
		return err
	}

	return c.ReconcileStorage(ctx, bytes.NewBuffer(content), &c.runtime.ControlRuntimeBootstrap)
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
