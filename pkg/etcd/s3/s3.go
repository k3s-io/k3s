package s3

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd/snapshot"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/lru"
)

var (
	clusterIDKey = textproto.CanonicalMIMEHeaderKey(version.Program + "-cluster-id")
	tokenHashKey = textproto.CanonicalMIMEHeaderKey(version.Program + "-token-hash")
	nodeNameKey  = textproto.CanonicalMIMEHeaderKey(version.Program + "-node-name")
)

var defaultEtcdS3 = &config.EtcdS3{
	Endpoint: "s3.amazonaws.com",
	Region:   "us-east-1",
	Timeout: metav1.Duration{
		Duration: 5 * time.Minute,
	},
}

var (
	controller *Controller
	cErr       error
	once       sync.Once
)

// Controller maintains state for S3 functionality,
// and can be used to get clients for interacting with
// an S3 service, given specific client configuration.
type Controller struct {
	clusterID   string
	tokenHash   string
	nodeName    string
	core        core.Interface
	clientCache *lru.Cache
}

// Client holds state for a given configuration - a preconfigured minio client,
// and reference to the config it was created for.
type Client struct {
	mc         *minio.Client
	etcdS3     *config.EtcdS3
	controller *Controller
}

// Start initializes the cache and sets the cluster id and token hash,
// returning a reference to the the initialized controller. Initialization is
// locked by a sync.Once to prevent races, and multiple calls to start will
// return the same controller or error.
func Start(ctx context.Context, config *config.Control) (*Controller, error) {
	once.Do(func() {
		c := &Controller{
			clientCache: lru.New(5),
			nodeName:    os.Getenv("NODE_NAME"),
		}

		if config.ClusterReset {
			logrus.Debug("Skip setting S3 snapshot cluster ID and server token hash during cluster-reset")
			controller = c
		} else {
			logrus.Debug("Getting S3 snapshot cluster ID and server token hash")
			if err := wait.PollImmediateUntilWithContext(ctx, time.Second, func(ctx context.Context) (bool, error) {
				if config.Runtime.Core == nil {
					return false, nil
				}
				c.core = config.Runtime.Core.Core()

				// cluster id hack: see https://groups.google.com/forum/#!msg/kubernetes-sig-architecture/mVGobfD4TpY/nkdbkX1iBwAJ
				ns, err := c.core.V1().Namespace().Get(metav1.NamespaceSystem, metav1.GetOptions{})
				if err != nil {
					return false, errors.Wrap(err, "failed to set S3 snapshot cluster ID")
				}
				c.clusterID = string(ns.UID)

				tokenHash, err := util.GetTokenHash(config)
				if err != nil {
					return false, errors.Wrap(err, "failed to set S3 snapshot server token hash")
				}
				c.tokenHash = tokenHash

				return true, nil
			}); err != nil {
				cErr = err
			} else {
				controller = c
			}
		}
	})

	return controller, cErr
}

func (c *Controller) GetClient(ctx context.Context, etcdS3 *config.EtcdS3) (*Client, error) {
	if etcdS3 == nil {
		return nil, errors.New("nil s3 configuration")
	}

	// update ConfigSecret in defaults so that comparisons between current and default config
	// ignore ConfigSecret when deciding if CLI configuration is present.
	defaultEtcdS3.ConfigSecret = etcdS3.ConfigSecret

	// If config is default, try to load config from secret, and fail if it cannot be retrieved or if the secret name is not set.
	// If config is not default, and secret name is set, warn that the secret is being ignored
	isDefault := reflect.DeepEqual(defaultEtcdS3, etcdS3)
	if etcdS3.ConfigSecret != "" {
		if isDefault {
			e, err := c.getConfigFromSecret(etcdS3.ConfigSecret)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get config from etcd-s3-config-secret %q", etcdS3.ConfigSecret)
			}
			logrus.Infof("Using etcd s3 configuration from etcd-s3-config-secret %q", etcdS3.ConfigSecret)
			etcdS3 = e
		} else {
			logrus.Warnf("Ignoring s3 configuration from etcd-s3-config-secret %q due to existing configuration from CLI or config file", etcdS3.ConfigSecret)
		}
	} else if isDefault {
		return nil, errors.New("s3 configuration was not set")
	}

	// used just for logging
	scheme := "https://"
	if etcdS3.Insecure {
		scheme = "http://"
	}

	// Try to get an existing client from cache.  The entire EtcdS3 struct
	// (including the key id and secret) is used as the cache key, but we only
	// print the endpoint and bucket name to avoid leaking creds into the logs.
	if client, ok := c.clientCache.Get(*etcdS3); ok {
		logrus.Infof("Reusing cached S3 client for endpoint=%q bucket=%q folder=%q", scheme+etcdS3.Endpoint, etcdS3.Bucket, etcdS3.Folder)
		return client.(*Client), nil
	}
	logrus.Infof("Attempting to create new S3 client for endpoint=%q bucket=%q folder=%q", scheme+etcdS3.Endpoint, etcdS3.Bucket, etcdS3.Folder)

	if etcdS3.Bucket == "" {
		return nil, errors.New("s3 bucket name was not set")
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()

	// You can either disable SSL verification or use a custom CA bundle,
	// it doesn't make sense to do both - if verification is disabled,
	// the CA is not checked!
	if etcdS3.SkipSSLVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	} else if etcdS3.EndpointCA != "" {
		tlsConfig, err := loadEndpointCAs(etcdS3.EndpointCA)
		if err != nil {
			return nil, err
		}
		tr.TLSClientConfig = tlsConfig
	}

	// Set a fixed proxy URL, if requested by the user. This replaces the default,
	// which calls ProxyFromEnvironment to read proxy settings from the environment.
	if etcdS3.Proxy != "" {
		var u *url.URL
		var err error
		// proxy address of literal "none" disables all use of a proxy by S3
		if etcdS3.Proxy != "none" {
			u, err = url.Parse(etcdS3.Proxy)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse etcd-s3-proxy value as URL")
			}
			if u.Scheme == "" || u.Host == "" {
				return nil, fmt.Errorf("proxy URL must include scheme and host")
			}
		}
		tr.Proxy = http.ProxyURL(u)
	}

	var creds *credentials.Credentials
	if len(etcdS3.AccessKey) == 0 && len(etcdS3.SecretKey) == 0 {
		creds = credentials.NewIAM("") // for running on ec2 instance
		if _, err := creds.Get(); err != nil {
			return nil, errors.Wrap(err, "failed to get IAM credentials")
		}
	} else {
		creds = credentials.NewStaticV4(etcdS3.AccessKey, etcdS3.SecretKey, "")
	}

	opt := minio.Options{
		Creds:        creds,
		Secure:       !etcdS3.Insecure,
		Region:       etcdS3.Region,
		Transport:    tr,
		BucketLookup: bucketLookupType(etcdS3.Endpoint),
	}
	mc, err := minio.New(etcdS3.Endpoint, &opt)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Checking if S3 bucket %s exists", etcdS3.Bucket)

	ctx, cancel := context.WithTimeout(ctx, etcdS3.Timeout.Duration)
	defer cancel()

	exists, err := mc.BucketExists(ctx, etcdS3.Bucket)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to test for existence of bucket %s", etcdS3.Bucket)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %s does not exist", etcdS3.Bucket)
	}
	logrus.Infof("S3 bucket %s exists", etcdS3.Bucket)

	client := &Client{
		mc:         mc,
		etcdS3:     etcdS3,
		controller: c,
	}
	logrus.Infof("Adding S3 client to cache")
	c.clientCache.Add(*etcdS3, client)
	return client, nil
}

// upload uploads the given snapshot to the configured S3
// compatible backend.
func (c *Client) Upload(ctx context.Context, snapshotPath string, extraMetadata *v1.ConfigMap, now time.Time) (*snapshot.File, error) {
	basename := filepath.Base(snapshotPath)
	metadata := filepath.Join(filepath.Dir(snapshotPath), "..", snapshot.MetadataDir, basename)
	snapshotKey := path.Join(c.etcdS3.Folder, basename)
	metadataKey := path.Join(c.etcdS3.Folder, snapshot.MetadataDir, basename)

	sf := &snapshot.File{
		Name:     basename,
		Location: fmt.Sprintf("s3://%s/%s", c.etcdS3.Bucket, snapshotKey),
		NodeName: "s3",
		CreatedAt: &metav1.Time{
			Time: now,
		},
		S3:             &snapshot.S3Config{EtcdS3: *c.etcdS3},
		Compressed:     strings.HasSuffix(snapshotPath, snapshot.CompressedExtension),
		MetadataSource: extraMetadata,
		NodeSource:     c.controller.nodeName,
	}

	logrus.Infof("Uploading snapshot to s3://%s/%s", c.etcdS3.Bucket, snapshotKey)
	uploadInfo, err := c.uploadSnapshot(ctx, snapshotKey, snapshotPath)
	if err != nil {
		sf.Status = snapshot.FailedStatus
		sf.Message = base64.StdEncoding.EncodeToString([]byte(err.Error()))
	} else {
		sf.Status = snapshot.SuccessfulStatus
		sf.Size = uploadInfo.Size
		sf.TokenHash = c.controller.tokenHash
	}
	if uploadInfo, err := c.uploadSnapshotMetadata(ctx, metadataKey, metadata); err != nil {
		logrus.Warnf("Failed to upload snapshot metadata to S3: %v", err)
	} else if uploadInfo.Size != 0 {
		logrus.Infof("Uploaded snapshot metadata s3://%s/%s", c.etcdS3.Bucket, metadataKey)
	}
	return sf, err
}

// uploadSnapshot uploads the snapshot file to S3 using the minio API.
func (c *Client) uploadSnapshot(ctx context.Context, key, path string) (info minio.UploadInfo, err error) {
	opts := minio.PutObjectOptions{
		NumThreads: 2,
		UserMetadata: map[string]string{
			clusterIDKey: c.controller.clusterID,
			nodeNameKey:  c.controller.nodeName,
			tokenHashKey: c.controller.tokenHash,
		},
	}
	if strings.HasSuffix(key, snapshot.CompressedExtension) {
		opts.ContentType = "application/zip"
	} else {
		opts.ContentType = "application/octet-stream"
	}
	ctx, cancel := context.WithTimeout(ctx, c.etcdS3.Timeout.Duration)
	defer cancel()
	return c.mc.FPutObject(ctx, c.etcdS3.Bucket, key, path, opts)
}

// uploadSnapshotMetadata marshals and uploads the snapshot metadata to S3 using the minio API.
// The upload is silently skipped if no extra metadata is provided.
func (c *Client) uploadSnapshotMetadata(ctx context.Context, key, path string) (info minio.UploadInfo, err error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return minio.UploadInfo{}, nil
		}
		return minio.UploadInfo{}, err
	}

	opts := minio.PutObjectOptions{
		NumThreads:  2,
		ContentType: "application/json",
		UserMetadata: map[string]string{
			clusterIDKey: c.controller.clusterID,
			nodeNameKey:  c.controller.nodeName,
			tokenHashKey: c.controller.tokenHash,
		},
	}
	ctx, cancel := context.WithTimeout(ctx, c.etcdS3.Timeout.Duration)
	defer cancel()
	return c.mc.FPutObject(ctx, c.etcdS3.Bucket, key, path, opts)
}

// Download downloads the given snapshot from the configured S3
// compatible backend. If the file is successfully downloaded, it returns
// the path the file was downloaded to.
func (c *Client) Download(ctx context.Context, snapshotName, snapshotDir string) (string, error) {
	snapshotKey := path.Join(c.etcdS3.Folder, snapshotName)
	metadataKey := path.Join(c.etcdS3.Folder, snapshot.MetadataDir, snapshotName)
	snapshotFile := filepath.Join(snapshotDir, snapshotName)
	metadataFile := filepath.Join(snapshotDir, "..", snapshot.MetadataDir, snapshotName)

	if err := c.downloadSnapshot(ctx, snapshotKey, snapshotFile); err != nil {
		return "", err
	}
	if err := c.downloadSnapshotMetadata(ctx, metadataKey, metadataFile); err != nil {
		return "", err
	}

	return snapshotFile, nil
}

// downloadSnapshot downloads the snapshot file from S3 using the minio API.
func (c *Client) downloadSnapshot(ctx context.Context, key, file string) error {
	logrus.Debugf("Downloading snapshot from s3://%s/%s", c.etcdS3.Bucket, key)
	ctx, cancel := context.WithTimeout(ctx, c.etcdS3.Timeout.Duration)
	defer cancel()
	defer os.Chmod(file, 0600)
	return c.mc.FGetObject(ctx, c.etcdS3.Bucket, key, file, minio.GetObjectOptions{})
}

// downloadSnapshotMetadata downloads the snapshot metadata file from S3 using the minio API.
// No error is returned if the metadata file does not exist, as it is optional.
func (c *Client) downloadSnapshotMetadata(ctx context.Context, key, file string) error {
	logrus.Debugf("Downloading snapshot metadata from s3://%s/%s", c.etcdS3.Bucket, key)
	ctx, cancel := context.WithTimeout(ctx, c.etcdS3.Timeout.Duration)
	defer cancel()
	defer os.Chmod(file, 0600)
	err := c.mc.FGetObject(ctx, c.etcdS3.Bucket, key, file, minio.GetObjectOptions{})
	if resp := minio.ToErrorResponse(err); resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

// SnapshotRetention prunes snapshots in the configured S3 compatible backend for this specific node.
// Returns a list of pruned snapshot names.
func (c *Client) SnapshotRetention(ctx context.Context, retention int, prefix string) ([]string, error) {
	if retention < 1 {
		return nil, nil
	}

	prefix = path.Join(c.etcdS3.Folder, prefix)
	logrus.Infof("Applying snapshot retention=%d to snapshots stored in s3://%s/%s", retention, c.etcdS3.Bucket, prefix)

	var snapshotFiles []minio.ObjectInfo

	toCtx, cancel := context.WithTimeout(ctx, c.etcdS3.Timeout.Duration)
	defer cancel()

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}
	for info := range c.mc.ListObjects(toCtx, c.etcdS3.Bucket, opts) {
		if info.Err != nil {
			return nil, info.Err
		}

		// skip metadata
		if path.Base(path.Dir(info.Key)) == snapshot.MetadataDir {
			continue
		}

		snapshotFiles = append(snapshotFiles, info)
	}

	if len(snapshotFiles) <= retention {
		return nil, nil
	}

	// sort newest-first so we can prune entries past the retention count
	sort.Slice(snapshotFiles, func(i, j int) bool {
		return snapshotFiles[j].LastModified.Before(snapshotFiles[i].LastModified)
	})

	deleted := []string{}
	for _, df := range snapshotFiles[retention:] {
		logrus.Infof("Removing S3 snapshot: s3://%s/%s", c.etcdS3.Bucket, df.Key)

		key := path.Base(df.Key)
		if err := c.DeleteSnapshot(ctx, key); err != nil && !snapshot.IsNotExist(err) {
			return deleted, err
		}
		deleted = append(deleted, key)
	}

	return deleted, nil
}

// DeleteSnapshot deletes the selected snapshot (and its metadata) from S3
func (c *Client) DeleteSnapshot(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, c.etcdS3.Timeout.Duration)
	defer cancel()

	key = path.Join(c.etcdS3.Folder, key)
	_, err := c.mc.StatObject(ctx, c.etcdS3.Bucket, key, minio.StatObjectOptions{})
	if err == nil {
		if err := c.mc.RemoveObject(ctx, c.etcdS3.Bucket, key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	// check for and try to delete the metadata regardless of whether or not the
	// snapshot existed, just to ensure that things are cleaned up in the case of
	// ephemeral errors. Metadata delete errors are only exposed if the object
	// exists and fails to delete.
	metadataKey := path.Join(path.Dir(key), snapshot.MetadataDir, path.Base(key))
	_, merr := c.mc.StatObject(ctx, c.etcdS3.Bucket, metadataKey, minio.StatObjectOptions{})
	if merr == nil {
		if err := c.mc.RemoveObject(ctx, c.etcdS3.Bucket, metadataKey, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	// return error from snapshot StatObject call, so that callers can determine
	// if the object was actually deleted or not by checking for a NotFound error.
	return err
}

// listSnapshots provides a list of currently stored
// snapshots in S3 along with their relevant
// metadata.
func (c *Client) ListSnapshots(ctx context.Context) (map[string]snapshot.File, error) {
	snapshots := map[string]snapshot.File{}
	metadatas := []string{}
	ctx, cancel := context.WithTimeout(ctx, c.etcdS3.Timeout.Duration)
	defer cancel()

	opts := minio.ListObjectsOptions{
		Prefix:    c.etcdS3.Folder,
		Recursive: true,
	}

	objects := c.mc.ListObjects(ctx, c.etcdS3.Bucket, opts)

	for obj := range objects {
		if obj.Err != nil {
			return nil, obj.Err
		}
		if obj.Size == 0 {
			continue
		}

		if o, err := c.mc.StatObject(ctx, c.etcdS3.Bucket, obj.Key, minio.StatObjectOptions{}); err != nil {
			logrus.Warnf("Failed to get object metadata: %v", err)
		} else {
			obj = o
		}

		filename := path.Base(obj.Key)
		if path.Base(path.Dir(obj.Key)) == snapshot.MetadataDir {
			metadatas = append(metadatas, obj.Key)
			continue
		}

		basename, compressed := strings.CutSuffix(filename, snapshot.CompressedExtension)
		ts, err := strconv.ParseInt(basename[strings.LastIndexByte(basename, '-')+1:], 10, 64)
		if err != nil {
			ts = obj.LastModified.Unix()
		}

		sf := snapshot.File{
			Name:     filename,
			Location: fmt.Sprintf("s3://%s/%s", c.etcdS3.Bucket, obj.Key),
			NodeName: "s3",
			CreatedAt: &metav1.Time{
				Time: time.Unix(ts, 0),
			},
			Size:       obj.Size,
			S3:         &snapshot.S3Config{EtcdS3: *c.etcdS3},
			Status:     snapshot.SuccessfulStatus,
			Compressed: compressed,
			NodeSource: obj.UserMetadata[nodeNameKey],
			TokenHash:  obj.UserMetadata[tokenHashKey],
		}
		sfKey := sf.GenerateConfigMapKey()
		snapshots[sfKey] = sf
	}

	for _, metadataKey := range metadatas {
		filename := path.Base(metadataKey)
		dsf := &snapshot.File{Name: filename, NodeName: "s3"}
		sfKey := dsf.GenerateConfigMapKey()
		if sf, ok := snapshots[sfKey]; ok {
			logrus.Debugf("Loading snapshot metadata from s3://%s/%s", c.etcdS3.Bucket, metadataKey)
			if obj, err := c.mc.GetObject(ctx, c.etcdS3.Bucket, metadataKey, minio.GetObjectOptions{}); err != nil {
				if snapshot.IsNotExist(err) {
					logrus.Debugf("Failed to get snapshot metadata: %v", err)
				} else {
					logrus.Warnf("Failed to get snapshot metadata for %s: %v", filename, err)
				}
			} else {
				if m, err := ioutil.ReadAll(obj); err != nil {
					if snapshot.IsNotExist(err) {
						logrus.Debugf("Failed to read snapshot metadata: %v", err)
					} else {
						logrus.Warnf("Failed to read snapshot metadata for %s: %v", filename, err)
					}
				} else {
					sf.Metadata = base64.StdEncoding.EncodeToString(m)
					snapshots[sfKey] = sf
				}
			}
		}
	}

	return snapshots, nil
}

func loadEndpointCAs(etcdS3EndpointCA string) (*tls.Config, error) {
	var loaded bool
	certPool := x509.NewCertPool()

	for _, ca := range strings.Split(etcdS3EndpointCA, " ") {
		// Try to decode the value as base64-encoded data - yes, a base64 string that itself
		// contains multiline, ascii-armored, base64-encoded certificate data - as would be produced
		// by `base64 --wrap=0 /path/to/cert.pem`. If this fails, assume the value is the path to a
		// file on disk, and try to read that.  This is backwards compatible with RKE1.
		caData, err := base64.StdEncoding.DecodeString(ca)
		if err != nil {
			caData, err = os.ReadFile(ca)
		}
		if err != nil {
			return nil, err
		}
		if certPool.AppendCertsFromPEM(caData) {
			loaded = true
		}
	}

	if loaded {
		return &tls.Config{RootCAs: certPool}, nil
	}
	return nil, errors.New("no certificates loaded from etcd-s3-endpoint-ca")
}

func bucketLookupType(endpoint string) minio.BucketLookupType {
	if strings.Contains(endpoint, "aliyun") { // backwards compatible with RKE1
		return minio.BucketLookupDNS
	}
	return minio.BucketLookupAuto
}
