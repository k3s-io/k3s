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
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
)

// Bootstrap attempts to load a managed database driver, if one has been initialized or should be created/joined.
// It then checks to see if the cluster needs to load bootstrap data, and if so, loads data into the
// ControlRuntimeBoostrap struct, either via HTTP or from the datastore.
func (c *Cluster) Bootstrap(ctx context.Context) error {
	if err := c.assignManagedDriver(ctx); err != nil {
		return err
	}

	return c.storageBootstrap(ctx)
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

// ReconcileBootstrapData is called before any data is saved to the
// datastore or locally. It checks to see if the contents of the
// bootstrap data in the datastore is newer than on disk or different
//  and dependingon where the difference is, the newer data is written
// to the older.
func (c *Cluster) ReconcileBootstrapData(ctx context.Context, r io.Reader, crb *config.ControlRuntimeBootstrap) error {
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
			logrus.Warn(info.Name() + " newer than within database")
			return c.save(ctx, true)
		case info.ModTime().Unix() < files[pathKey].Timestamp.Unix():
			logrus.Warn("database newer than " + info.Name())
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
