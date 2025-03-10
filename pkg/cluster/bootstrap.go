package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-test/deep"
	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/k3s-io/kine/pkg/client"
	"github.com/k3s-io/kine/pkg/endpoint"
	"github.com/otiai10/copy"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Bootstrap attempts to load a managed database driver, if one has been initialized or should be created/joined.
// It then checks to see if the cluster needs to load bootstrap data, and if so, loads data into the
// ControlRuntimeBootstrap struct, either via HTTP or from the datastore.
func (c *Cluster) Bootstrap(ctx context.Context, clusterReset bool) error {
	if err := c.assignManagedDriver(ctx); err != nil {
		return pkgerrors.WithMessage(err, "failed to set datastore driver")
	}

	// Check if we need to bootstrap, and whether or not the managed database has already
	// been initialized (created or joined an existing cluster). Note that nodes without
	// a local datastore always need to bootstrap and never count as initialized.
	// This also sets c.clientAccessInfo if c.config.JoinURL and c.config.Token are set.
	shouldBootstrap, isInitialized, err := c.shouldBootstrapLoad(ctx)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to check if bootstrap data has been initialized")
	}

	if c.managedDB != nil {
		if c.config.DisableETCD {
			// secondary server with etcd disabled, start the etcd proxy so that we can attempt to use it
			// when reconciling.
			if err := c.startEtcdProxy(ctx); err != nil {
				return pkgerrors.WithMessage(err, "failed to start etcd proxy")
			}
		} else if isInitialized && !clusterReset {
			// For secondary servers with etcd, first attempt to connect and reconcile using the join URL.
			// This saves on having to start up a temporary etcd just to extract bootstrap data.
			if c.clientAccessInfo != nil {
				if err := c.httpBootstrap(ctx); err != nil {
					logrus.Warnf("Unable to reconcile with remote datastore: %v", err)
				} else {
					logrus.Info("Successfully reconciled with remote datastore")
					return nil
				}
			}
			// Not a secondary server or failed to reconcile via join URL, start up a temporary etcd
			// with the local datastore and use that to reconcile.
			if err := c.reconcileEtcd(ctx); err != nil {
				logrus.Fatalf("Failed to reconcile with temporary etcd: %v", err)
			}
		}
	}

	if shouldBootstrap {
		return c.bootstrap(ctx)
	}

	return nil
}

// shouldBootstrapLoad returns true if we need to load ControlRuntimeBootstrap data again and a
// second boolean indicating that the server has or has not been initialized, if etcd. This is
// controlled by a stamp file on disk that records successful bootstrap using a hash of the join
// token. This function also sets up the HTTP Bootstrap request handler and sets
// c.clientAccessInfo if join url and token are set.
func (c *Cluster) shouldBootstrapLoad(ctx context.Context) (bool, bool, error) {
	opts := []clientaccess.ValidationOption{
		clientaccess.WithUser("server"),
		clientaccess.WithCACertificate(c.config.Runtime.ServerCA),
	}

	// Non-nil managedDB indicates that the database is either initialized, initializing, or joining
	if c.managedDB != nil {
		c.config.Runtime.HTTPBootstrap = c.serveBootstrap()
		isInitialized, err := c.managedDB.IsInitialized()
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
				c.clientAccessInfo, _ = clientaccess.ParseAndValidateToken(c.config.JoinURL, c.config.Token, opts...)
			}
			return false, true, nil
		} else if c.config.JoinURL == "" {
			// Not initialized, not joining - must be initializing (cluster-init)
			logrus.Infof("Managed %s cluster initializing", c.managedDB.EndpointName())
			return false, false, nil
		} else {
			// Not initialized, but have a Join URL - fail if there's no token; if there is then validate it.
			// Note that this is the path taken by control-plane-only nodes every startup, as they have a non-nil managedDB that is never initialized.
			if c.config.Token == "" {
				return false, false, errors.New("token is required to join a cluster")
			}

			// Fail if the token isn't syntactically valid, or if the CA hash on the remote server doesn't match
			// the hash in the token. The password isn't actually checked until later when actually bootstrapping.
			info, err := clientaccess.ParseAndValidateToken(c.config.JoinURL, c.config.Token, opts...)
			if err != nil {
				return false, false, pkgerrors.WithMessage(err, "failed to validate token")
			}
			c.clientAccessInfo = info

			if c.config.DisableETCD {
				logrus.Infof("Managed %s disabled on this node", c.managedDB.EndpointName())
			} else {
				logrus.Infof("Managed %s cluster not yet initialized", c.managedDB.EndpointName())
			}
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
			tokenFromFile, err := util.ReadTokenFromFile(c.config.Runtime.ServerToken, c.config.Runtime.ServerCA, c.config.DataDir)
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

		normalizedToken, err := util.NormalizeToken(token)
		if err != nil {
			return err
		}

		var value *client.Value

		storageClient, err := client.New(c.config.Runtime.EtcdConfig)
		if err != nil {
			return err
		}
		defer storageClient.Close()

		value, c.saveBootstrap, err = getBootstrapKeyFromStorage(ctx, storageClient, normalizedToken, token)
		if err != nil {
			return err
		}
		if value == nil {
			return nil
		}

		dbRawData, err = decrypt(normalizedToken, value.Data)
		if err != nil {
			return err
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

	// Compare on-disk content to the datastore.
	// If the files differ and the timestamp in the datastore is newer, data on disk will be updated.
	// If the files differ and the timestamp on disk is newer, an error will be raised listing the conflicting files.
	var updateDisk bool
	var newerOnDisk []string
	for pathKey, fileData := range files {
		path, ok := paths[pathKey]
		if !ok || path == "" {
			logrus.Warnf("Unable to lookup path to reconcile %s", pathKey)
			continue
		}
		logrus.Debugf("Reconciling %s at '%s'", pathKey, path)

		updated, newer, err := isNewerFile(path, fileData)
		if err != nil {
			return pkgerrors.WithMessagef(err, "failed to get update status of %s", pathKey)
		}
		if newer {
			newerOnDisk = append(newerOnDisk, path)
		}

		updateDisk = updateDisk || updated
	}

	if c.config.ClusterReset {
		updateDisk = true
		serverTLSDir := filepath.Join(c.config.DataDir, "tls")
		tlsBackupDir := filepath.Join(c.config.DataDir, "tls-"+strconv.Itoa(int(time.Now().Unix())))

		logrus.Infof("Cluster reset: backing up certificates directory to " + tlsBackupDir)

		if _, err := os.Stat(serverTLSDir); err != nil {
			return pkgerrors.WithMessage(err, "cluster reset failed to stat server TLS dir")
		}
		if err := copy.Copy(serverTLSDir, tlsBackupDir); err != nil {
			return pkgerrors.WithMessage(err, "cluster reset failed to back up server TLS dir")
		}
	} else if len(newerOnDisk) > 0 {
		logrus.Fatal(strings.Join(newerOnDisk, ", ") + " newer than datastore and could cause a cluster outage. Remove the file(s) from disk and restart to be recreated from datastore.")
	}

	if updateDisk {
		logrus.Warn("Updating bootstrap data on disk from datastore")
		return bootstrap.WriteToDiskFromStorage(files, crb)
	}

	return nil
}

// isNewerFile compares the file from disk and datastore, and returns
// update status.
func isNewerFile(path string, file bootstrap.File) (updated bool, newerOnDisk bool, _ error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Warn(path + " doesn't exist. continuing...")
			return true, false, nil
		}
		return false, false, pkgerrors.WithMessagef(err, "reconcile failed to open")
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return false, false, pkgerrors.WithMessagef(err, "reconcile failed to read")
	}

	if bytes.Equal(file.Content, data) {
		return false, false, nil
	}

	info, err := f.Stat()
	if err != nil {
		return false, false, pkgerrors.WithMessagef(err, "reconcile failed to stat")
	}

	if info.ModTime().Unix()-file.Timestamp.Unix() >= systemTimeSkew {
		return true, true, nil
	}

	logrus.Warn(path + " will be updated from the datastore.")
	return true, false, nil
}

// serveBootstrap sends bootstrap data to the client, a server that is joining the cluster and
// has only a server token, and cannot use CA certs/keys to access the datastore directly.
func (c *Cluster) serveBootstrap() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		// Try getting data from the datastore first. Token has already been validated by the request handler.
		_, token, _ := req.BasicAuth()
		data, err := c.getBootstrapData(req.Context(), token)
		if err != nil {
			// If we failed to read data from the datastore, just send data from disk.
			logrus.Warnf("Failed to retrieve HTTP bootstrap data from datastore; falling back to disk for %s: %v", req.RemoteAddr, err)
			bootstrap.ReadFromDisk(rw, &c.config.Runtime.ControlRuntimeBootstrap)
			return
		}
		logrus.Infof("Serving HTTP bootstrap from datastore for %s", req.RemoteAddr)
		rw.Write(data)
	})
}

// httpBootstrap retrieves bootstrap data (certs and keys, etc) from the remote server via HTTP
// and loads it into the ControlRuntimeBootstrap struct. Unlike the storage bootstrap path,
// this data does not need to be decrypted since it is generated on-demand by an existing server.
func (c *Cluster) httpBootstrap(ctx context.Context) error {
	content, err := c.clientAccessInfo.Get("/v1-"+version.Program+"/server-bootstrap", clientaccess.WithTimeout(15*time.Second))
	if err != nil {
		return err
	}

	return c.ReconcileBootstrapData(ctx, bytes.NewReader(content), &c.config.Runtime.ControlRuntimeBootstrap, true)
}

// readBootstrapFromDisk returns a buffer holding the JSON-serialized bootstrap data read from disk.
func (c *Cluster) readBootstrapFromDisk() (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if err := bootstrap.ReadFromDisk(&buf, &c.config.Runtime.ControlRuntimeBootstrap); err != nil {
		return nil, err
	}

	return &buf, nil
}

// bootstrap retrieves cluster bootstrap data: CA certs and other common config. This uses HTTP
// for etcd (as this node does not yet have CA data available), and direct load from datastore
// when using kine.
func (c *Cluster) bootstrap(ctx context.Context) error {
	c.joining = true

	if c.managedDB != nil {
		// Try to compare local config against the server we're joining.
		if err := c.compareConfig(); err != nil {
			return pkgerrors.WithMessage(err, "failed to validate server configuration")
		}
		// Try to bootstrap from the datastore using the local etcd proxy.
		if data, err := c.getBootstrapData(ctx, c.clientAccessInfo.Password); err != nil {
			logrus.Debugf("Failed to get bootstrap data from etcd proxy: %v", err)
		} else {
			if err := c.ReconcileBootstrapData(ctx, bytes.NewReader(data), &c.config.Runtime.ControlRuntimeBootstrap, false); err != nil {
				logrus.Debugf("Failed to reconcile bootstrap data from etcd proxy: %v", err)
			} else {
				return nil
			}
		}
		// fall back to bootstrapping from the join URL
		return c.httpBootstrap(ctx)
	}

	// Bootstrap directly from datastore
	return c.storageBootstrap(ctx)
}

// compareConfig verifies that the config of the joining control plane node coincides with the cluster's config
func (c *Cluster) compareConfig() error {
	opts := []clientaccess.ValidationOption{
		clientaccess.WithUser("node"),
		clientaccess.WithCACertificate(c.config.Runtime.ServerCA),
	}

	token := c.config.AgentToken
	if token == "" {
		token = c.config.Token
	}
	agentClientAccessInfo, err := clientaccess.ParseAndValidateToken(c.config.JoinURL, token, opts...)
	if err != nil {
		return err
	}
	serverConfig, err := agentClientAccessInfo.Get("/v1-" + version.Program + "/config")
	if err != nil {
		logrus.Warnf("Skipping cluster configuration validation: %v", err)
		return nil
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

	if diff := deep.Equal(c.config.CriticalControlArgs, clusterControl.CriticalControlArgs); diff != nil {
		rc := reflect.ValueOf(clusterControl.CriticalControlArgs).Type()
		for _, d := range diff {
			field := strings.Split(d, ":")[0]
			v, _ := rc.FieldByName(field)
			if cliTag, found := v.Tag.Lookup("cli"); found {
				logrus.Warnf("critical configuration mismatched: %s", cliTag)
			} else {
				logrus.Warnf("critical configuration mismatched: %s", field)
			}
		}
		return errors.New("critical configuration value mismatch between servers")
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
	if err := e.SetControlConfig(c.config); err != nil {
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

	data, err := c.readBootstrapFromDisk()
	if err != nil {
		return err
	}

	return c.ReconcileBootstrapData(reconcileCtx, bytes.NewReader(data.Bytes()), &c.config.Runtime.ControlRuntimeBootstrap, false)
}
