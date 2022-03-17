package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/k3s-io/kine/pkg/client"
	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Bootstrap attempts to load a managed database driver, if one has been initialized or should be created/joined.
// It then checks to see if the cluster needs to load bootstrap data, and if so, loads data into the
// ControlRuntimeBoostrap struct, either via HTTP or from the datastore.
func (c *Cluster) Bootstrap(ctx context.Context, snapshot bool) error {
	if err := c.assignManagedDriver(ctx); err != nil {
		return err
	}

	shouldBootstrap, isInitialized, err := c.shouldBootstrapLoad(ctx)
	if err != nil {
		return err
	}
	c.shouldBootstrap = shouldBootstrap

	if c.managedDB != nil {
		if !snapshot {
			isHTTP := c.config.JoinURL != "" && c.config.Token != ""
			// For secondary servers, we attempt to connect and reconcile with the datastore.
			// If that fails we fallback to the local etcd cluster start
			if isInitialized && isHTTP && c.clientAccessInfo != nil {
				if err := c.httpBootstrap(ctx); err == nil {
					logrus.Info("Successfully reconciled with datastore")
					return nil
				}
				logrus.Warnf("Unable to reconcile with datastore: %v", err)
			}
			// In the case of etcd, if the database has been initialized, it doesn't
			// need to be bootstrapped however we still need to check the database
			// and reconcile the bootstrap data. Below we're starting a temporary
			// instance of etcd in the event that etcd certificates are unavailable,
			// reading the data, and comparing that to the data on disk, all the while
			// starting normal etcd.
			if isInitialized {
				if err := c.reconcileEtcd(ctx); err != nil {
					logrus.Fatalf("Failed to reconcile with temporary etcd: %v", err)
				}
			}
		}
	}

	if c.shouldBootstrap {
		return c.bootstrap(ctx)
	}

	return nil
}

// shouldBootstrapLoad returns true if we need to load ControlRuntimeBootstrap data again and a second boolean
// indicating that the server has or has not been initialized, if etcd. This is controlled by a stamp file on
// disk that records successful bootstrap using a hash of the join token.
func (c *Cluster) shouldBootstrapLoad(ctx context.Context) (bool, bool, error) {
	// Non-nil managedDB indicates that the database is either initialized, initializing, or joining
	if c.managedDB != nil {
		c.config.Runtime.HTTPBootstrap = true

		isInitialized, err := c.managedDB.IsInitialized(ctx, c.config)
		if err != nil {
			return false, false, err
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
			return false, true, nil
		} else if c.config.JoinURL == "" {
			// Not initialized, not joining - must be initializing (cluster-init)
			logrus.Infof("Managed %s cluster initializing", c.managedDB.EndpointName())
			return false, false, nil
		} else {
			// Not initialized, but have a Join URL - fail if there's no token; if there is then validate it.
			if c.config.Token == "" {
				return false, false, errors.New(version.ProgramUpper + "_TOKEN is required to join a cluster")
			}

			// Fail if the token isn't syntactically valid, or if the CA hash on the remote server doesn't match
			// the hash in the token. The password isn't actually checked until later when actually bootstrapping.
			info, err := clientaccess.ParseAndValidateTokenForUser(c.config.JoinURL, c.config.Token, "server")
			if err != nil {
				return false, false, err
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
		"cred",
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
func migrateBootstrapData(ctx context.Context, data io.Reader, files bootstrap.PathsDataformat) error {
	logrus.Info("Migrating bootstrap data to new format")

	var oldBootstrapData map[string][]byte
	if err := json.NewDecoder(data).Decode(&oldBootstrapData); err != nil {
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

	return nil
}

const systemTimeSkew = int64(3)

// isMigrated checks to see if the given bootstrap data
// is in the latest format.
func isMigrated(buf io.ReadSeeker, files *bootstrap.PathsDataformat) bool {
	buf.Seek(0, 0)
	defer buf.Seek(0, 0)

	if err := json.NewDecoder(buf).Decode(files); err != nil {
		// This will fail if data is being pulled from old an cluster since
		// older clusters used a map[string][]byte for the data structure.
		// Therefore, we need to perform a migration to the newer bootstrap
		// format; bootstrap.BootstrapFile.
		return false
	}

	return true
}

// ReconcileBootstrapData is called before any data is saved to the
// datastore or locally. It checks to see if the contents of the
// bootstrap data in the datastore is newer than on disk or different
// and depending on where the difference is. If the datastore is newer,
// then the data will be written to disk. If the data on disk is newer,
// k3s will exit with an error.
func (c *Cluster) ReconcileBootstrapData(ctx context.Context, buf io.ReadSeeker, crb *config.ControlRuntimeBootstrap, isHTTP bool) error {
	logrus.Info("Reconciling bootstrap data between datastore and disk")

	if err := c.certDirsExist(); err != nil {
		// we need to see if the data has been migrated before writing to disk. This
		// is because the data may have been given to us via the HTTP bootstrap process
		// from an older version of k3s. That version might not have the new data format
		// and we should write the correct format.
		files := make(bootstrap.PathsDataformat)
		if !isMigrated(buf, &files) {
			if err := migrateBootstrapData(ctx, buf, files); err != nil {
				return err
			}
			buf.Seek(0, 0)
		}

		logrus.Debugf("One or more certificate directories do not exist; writing data to disk from datastore")
		return bootstrap.WriteToDiskFromStorage(files, crb)
	}

	var dbRawData []byte
	if c.managedDB != nil && !isHTTP {
		token := c.config.Token
		if token == "" {
			tokenFromFile, err := readTokenFromFile(c.config.Runtime.ServerToken, c.config.Runtime.ServerCA, c.config.DataDir)
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

		storageClient, err := client.New(c.config.Runtime.EtcdConfig)
		if err != nil {
			return err
		}
		defer storageClient.Close()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

	RETRY:
		for {
			value, c.saveBootstrap, err = getBootstrapKeyFromStorage(ctx, storageClient, normalizedToken, token)
			if err != nil {
				if strings.Contains(err.Error(), "not supported for learner") {
					for range ticker.C {
						continue RETRY
					}

				}
				return err
			}
			if value == nil {
				return nil
			}

			dbRawData, err = decrypt(normalizedToken, value.Data)
			if err != nil {
				return err
			}

			break
		}

		buf = bytes.NewReader(dbRawData)
	}

	paths, err := bootstrap.ObjToMap(crb)
	if err != nil {
		return err
	}

	files := make(bootstrap.PathsDataformat)
	if !isMigrated(buf, &files) {
		if err := migrateBootstrapData(ctx, buf, files); err != nil {
			return err
		}
		buf.Seek(0, 0)
	}

	type update struct {
		db, disk, conflict bool
	}

	var updateDisk bool

	results := make(map[string]update)
	for pathKey, fileData := range files {
		path, ok := paths[pathKey]
		if !ok || path == "" {
			logrus.Warnf("Unable to lookup path to reconcile %s", pathKey)
			continue
		}
		logrus.Debugf("Reconciling %s at '%s'", pathKey, path)

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

	var newerOnDisk []string
	for path, res := range results {
		switch {
		case res.disk:
			updateDisk = true
			logrus.Warn("Datastore newer than " + path)
		case res.db:
			if c.config.ClusterReset {
				logrus.Infof("Cluster reset: replacing file on disk: " + path)
				updateDisk = true
				continue
			}
			newerOnDisk = append(newerOnDisk, path)
		case res.conflict:
			logrus.Warnf("Datastore / disk conflict: %s newer than in the datastore", path)
		}
	}

	if len(newerOnDisk) > 0 {
		logrus.Fatal(strings.Join(newerOnDisk, ", ") + " newer than datastore and could cause a cluster outage. Remove the file(s) from disk and restart to be recreated from datastore.")
	}

	if updateDisk {
		logrus.Warn("Updating bootstrap data on disk from datastore")
		return bootstrap.WriteToDiskFromStorage(files, crb)
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

	return c.ReconcileBootstrapData(ctx, bytes.NewReader(content), &c.config.Runtime.ControlRuntimeBootstrap, true)
}

func (c *Cluster) retrieveInitializedDBdata(ctx context.Context) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if err := bootstrap.ReadFromDisk(&buf, &c.config.Runtime.ControlRuntimeBootstrap); err != nil {
		return nil, err
	}

	return &buf, nil
}

// bootstrap performs cluster bootstrapping, either via HTTP (for managed databases) or direct load from datastore.
func (c *Cluster) bootstrap(ctx context.Context) error {
	c.joining = true

	// bootstrap managed database via HTTPS
	if c.config.Runtime.HTTPBootstrap {
		// Assuming we should just compare on managed databases
		if err := c.compareConfig(); err != nil {
			return errors.Wrap(err, "failed to validate server configuration")
		}
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

// compareConfig verifies that the config of the joining control plane node coincides with the cluster's config
func (c *Cluster) compareConfig() error {
	token := c.config.AgentToken
	if token == "" {
		token = c.config.Token
	}
	agentClientAccessInfo, err := clientaccess.ParseAndValidateTokenForUser(c.config.JoinURL, token, "node")
	if err != nil {
		return err
	}
	serverConfig, err := agentClientAccessInfo.Get("/v1-" + version.Program + "/config")
	if err != nil {
		return err
	}
	clusterControl := &config.Control{}
	if err := json.Unmarshal(serverConfig, clusterControl); err != nil {
		return err
	}

	// We are saving IPs of ClusterIPRanges and ServiceIPRanges in 4-bytes representation but json decodes in 16-byte
	ipsTo16Bytes(c.config.CriticalControlArgs.ClusterIPRanges)
	ipsTo16Bytes(c.config.CriticalControlArgs.ServiceIPRanges)

	// If the remote server is down-level and did not fill the egress-selector
	// mode, use the local value to allow for temporary mismatch during upgrades.
	if clusterControl.CriticalControlArgs.EgressSelectorMode == "" {
		clusterControl.CriticalControlArgs.EgressSelectorMode = c.config.CriticalControlArgs.EgressSelectorMode
	}

	if !reflect.DeepEqual(clusterControl.CriticalControlArgs, c.config.CriticalControlArgs) {
		logrus.Debugf("This is the server CriticalControlArgs: %#v", clusterControl.CriticalControlArgs)
		logrus.Debugf("This is the local CriticalControlArgs: %#v", c.config.CriticalControlArgs)
		return errors.New("critical configuration value mismatch")
	}
	return nil
}

// ipsTo16Bytes makes sure the IPs in the []*net.IPNet slice are represented in 16-byte format
func ipsTo16Bytes(mySlice []*net.IPNet) {
	for _, ipNet := range mySlice {
		ipNet.IP = ipNet.IP.To16()
	}
}

// reconcileEtcd starts a temporary single-member etcd cluster using a copy of the
// etcd database, and uses it to reconcile bootstrap data. This is necessary
// because the full etcd cluster may not have quorum during startup, but we still
// need to extract data from the datastore.
func (c *Cluster) reconcileEtcd(ctx context.Context) error {
	logrus.Info("Starting temporary etcd to reconcile with datastore")

	tempConfig := endpoint.ETCDConfig{Endpoints: []string{"http://127.0.0.1:2399"}}
	originalConfig := c.config.Runtime.EtcdConfig
	c.config.Runtime.EtcdConfig = tempConfig
	reconcileCtx, cancel := context.WithCancel(ctx)

	defer func() {
		cancel()
		c.config.Runtime.EtcdConfig = originalConfig
	}()

	e := etcd.NewETCD()
	if err := e.SetControlConfig(reconcileCtx, c.config); err != nil {
		return err
	}
	if err := e.StartEmbeddedTemporary(reconcileCtx); err != nil {
		return err
	}

	for {
		if err := e.Test(reconcileCtx); err != nil && !errors.Is(err, etcd.ErrNotMember) {
			logrus.Infof("Failed to test temporary data store connection: %v", err)
		} else {
			logrus.Info(e.EndpointName() + " temporary data store connection OK")
			break
		}

		select {
		case <-time.After(5 * time.Second):
		case <-reconcileCtx.Done():
			break
		}
	}

	data, err := c.retrieveInitializedDBdata(reconcileCtx)
	if err != nil {
		return err
	}

	return c.ReconcileBootstrapData(reconcileCtx, bytes.NewReader(data.Bytes()), &c.config.Runtime.ControlRuntimeBootstrap, false)
}
