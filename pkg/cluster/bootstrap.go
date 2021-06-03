package cluster

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
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

	if err := c.validateBootstrapCertificates(); err != nil {
		return true, nil
	}

	// Check the stamp file to see if we have successfully bootstrapped using this token.
	// NOTE: The fact that we use a hash of the token to generate the stamp
	//       means that it is unsafe to use the same token for multiple clusters.
	stamp := c.bootstrapStamp()
	if _, err := os.Stat(stamp); err == nil {
		logrus.Info("Cluster bootstrap already complete")
		return true, nil
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

func (c *Cluster) validateBootstrapCertificates() error {
	bootstrapDirs := []string{
		"tls",
		"tls/etcd",
	}

	for _, dir := range bootstrapDirs {
		if _, err := os.Stat(filepath.Join(c.config.DataDir, dir)); os.IsNotExist(err) {
			logrus.Debugf("missing %s directory from ${data-dir}", dir)
			return fmt.Errorf("missing %s directory from ${data-dir}", dir)
		}

		ok, err := isDirEmpty(filepath.Join(c.config.DataDir, dir))
		if err != nil {
			return err
		}
		if ok {
			logrus.Debugf("%s directory is empty", dir)
			return fmt.Errorf("%s directory is empty", dir)
		}
	}

	// check existence of certificate and contents against known bootstrap data and
	// if there are any differences, return an error to trigger a rebootstrap
	bootstrapCertificateAndFile := map[string]string{
		c.config.Runtime.ControlRuntimeBootstrap.ETCDServerCA:    "etcd/server-ca.crt",
		c.config.Runtime.ControlRuntimeBootstrap.ETCDServerCAKey: "etcd/server-ca.key",
		c.config.Runtime.ControlRuntimeBootstrap.ETCDPeerCA:      "etcd/peer-ca.crt",
		c.config.Runtime.ControlRuntimeBootstrap.ETCDPeerCAKey:   "etcd/peer-ca.key",
		c.config.Runtime.ClientETCDCert:                          "etcd/client.crt",
		c.config.Runtime.ClientETCDKey:                           "etcd/client.key",
		c.config.Runtime.PeerServerClientETCDCert:                "etcd/peer-server-client.crt",
		c.config.Runtime.PeerServerClientETCDKey:                 "etcd/peer-server-client.key",
		c.config.Runtime.ServerETCDCert:                          "etcd/server-client.crt",
		c.config.Runtime.ServerETCDKey:                           "etcd/server-client.key",

		c.config.Runtime.ClientAdminCert:                  "client-admin.crt",
		c.config.Runtime.ClientAdminKey:                   "client-admin.key",
		c.config.Runtime.ClientAuthProxyCert:              "client-auth-proxy.crt",
		c.config.Runtime.ClientAuthProxyKey:               "client-auth-proxy.key",
		c.config.Runtime.ClientCA:                         "client-ca.crt",
		c.config.Runtime.ClientCAKey:                      "client-ca.key",
		c.config.Runtime.ClientCloudControllerCert:        "client-cloud-controller.crt",
		c.config.Runtime.ClientCloudControllerKey:         "client-cloud-controller.key",
		c.config.Runtime.ClientControllerCert:             "client-controller.crt",
		c.config.Runtime.ClientControllerKey:              "client-controller.key",
		c.config.Runtime.ClientK3sControllerCert:          "client-k3s-controller.crt",
		c.config.Runtime.ClientK3sControllerKey:           "client-k3s-controller.key",
		c.config.Runtime.ClientKubeAPICert:                "client-kube-apiserver.crt",
		c.config.Runtime.ClientKubeAPIKey:                 "client-kube-apiserver.key",
		c.config.Runtime.ClientKubeletKey:                 "client-kubelet.key",
		c.config.Runtime.ClientKubeProxyCert:              "client-kube-proxy.crt",
		c.config.Runtime.ClientKubeProxyKey:               "client-kube-proxy.key",
		c.config.Runtime.ClientSchedulerCert:              "client-scheduler.crt",
		c.config.Runtime.ClientSchedulerKey:               "client-scheduler.key",
		c.config.Runtime.ControlRuntimeBootstrap.ServerCA: "dynamic-cert.json", // ?
		c.config.Runtime.RequestHeaderCA:                  "request-header-ca.crt",
		c.config.Runtime.RequestHeaderCAKey:               "request-header-ca.key",
		c.config.Runtime.ServerCA:                         "server-ca.crt",
		c.config.Runtime.ServerCAKey:                      "server-ca.key",
		c.config.Runtime.ServiceKey:                       "service.key",
		c.config.Runtime.ServingKubeAPICert:               "serving-kube-apiserver.crt",
		c.config.Runtime.ServingKubeAPIKey:                "serving-kube-apiserver.key",
		c.config.Runtime.ServingKubeletKey:                "serving-kubelet.key",
	}

	for content, filename := range bootstrapCertificateAndFile {
		if err := c.checkBootstrapCertificate(content, filename); err != nil {
			return err
		}
	}

	return nil
}

// checkBootstrapCertificate checks the given content string against what's
// read in the file given, hashes them, and compares the hashes. If they don't
// match an error is returned.
func (c *Cluster) checkBootstrapCertificate(content, filename string) error {
	bootstrapHash := sha256.Sum256([]byte(content))

	fileHash := sha256.New()
	f, err := os.Open(filepath.Join(c.config.DataDir, "tls", filename))
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(fileHash, f); err != nil {
		return err
	}

	if ret := bytes.Compare(bootstrapHash[:], fileHash.Sum(nil)); ret != 0 {
		return fmt.Errorf("%s doesn't match bootstrap data", filename)
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
func (c *Cluster) httpBootstrap() error {
	content, err := c.clientAccessInfo.Get("/v1-" + version.Program + "/server-bootstrap")
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
