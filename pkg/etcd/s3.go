package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/textproto"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	clusterIDKey = textproto.CanonicalMIMEHeaderKey(version.Program + "-cluster-id")
	tokenHashKey = textproto.CanonicalMIMEHeaderKey(version.Program + "-token-hash")
	nodeNameKey  = textproto.CanonicalMIMEHeaderKey(version.Program + "-node-name")
)

// S3 maintains state for S3 functionality.
type S3 struct {
	config    *config.Control
	client    *minio.Client
	clusterID string
	tokenHash string
	nodeName  string
}

// newS3 creates a new value of type s3 pointer with a
// copy of the config.Control pointer and initializes
// a new Minio client.
func NewS3(ctx context.Context, config *config.Control) (*S3, error) {
	if config.EtcdS3BucketName == "" {
		return nil, errors.New("s3 bucket name was not set")
	}
	tr := http.DefaultTransport

	switch {
	case config.EtcdS3EndpointCA != "":
		trCA, err := setTransportCA(tr, config.EtcdS3EndpointCA, config.EtcdS3SkipSSLVerify)
		if err != nil {
			return nil, err
		}
		tr = trCA
	case config.EtcdS3 && config.EtcdS3SkipSSLVerify:
		tr.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: config.EtcdS3SkipSSLVerify,
		}
	}

	var creds *credentials.Credentials
	if len(config.EtcdS3AccessKey) == 0 && len(config.EtcdS3SecretKey) == 0 {
		creds = credentials.NewIAM("") // for running on ec2 instance
	} else {
		creds = credentials.NewStaticV4(config.EtcdS3AccessKey, config.EtcdS3SecretKey, "")
	}

	opt := minio.Options{
		Creds:        creds,
		Secure:       !config.EtcdS3Insecure,
		Region:       config.EtcdS3Region,
		Transport:    tr,
		BucketLookup: bucketLookupType(config.EtcdS3Endpoint),
	}
	c, err := minio.New(config.EtcdS3Endpoint, &opt)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Checking if S3 bucket %s exists", config.EtcdS3BucketName)

	ctx, cancel := context.WithTimeout(ctx, config.EtcdS3Timeout)
	defer cancel()

	exists, err := c.BucketExists(ctx, config.EtcdS3BucketName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to test for existence of bucket %s", config.EtcdS3BucketName)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %s does not exist", config.EtcdS3BucketName)
	}
	logrus.Infof("S3 bucket %s exists", config.EtcdS3BucketName)

	s3 := &S3{
		config:   config,
		client:   c,
		nodeName: os.Getenv("NODE_NAME"),
	}

	if config.ClusterReset {
		logrus.Debug("Skip setting S3 snapshot cluster ID and token during cluster-reset")
	} else {
		if err := wait.PollImmediateUntilWithContext(ctx, time.Second, func(ctx context.Context) (bool, error) {
			if config.Runtime.Core == nil {
				return false, nil
			}

			// cluster id hack: see https://groups.google.com/forum/#!msg/kubernetes-sig-architecture/mVGobfD4TpY/nkdbkX1iBwAJ
			ns, err := config.Runtime.Core.Core().V1().Namespace().Get(metav1.NamespaceSystem, metav1.GetOptions{})
			if err != nil {
				return false, errors.Wrap(err, "failed to set S3 snapshot cluster ID")
			}
			s3.clusterID = string(ns.UID)

			tokenHash, err := util.GetTokenHash(config)
			if err != nil {
				return false, errors.Wrap(err, "failed to set S3 snapshot server token hash")
			}
			s3.tokenHash = tokenHash

			return true, nil
		}); err != nil {
			return nil, err
		}
	}

	return s3, nil
}

// upload uploads the given snapshot to the configured S3
// compatible backend.
func (s *S3) upload(ctx context.Context, snapshot string, extraMetadata *v1.ConfigMap, now time.Time) (*snapshotFile, error) {
	basename := filepath.Base(snapshot)
	metadata := filepath.Join(filepath.Dir(snapshot), "..", metadataDir, basename)
	snapshotKey := path.Join(s.config.EtcdS3Folder, basename)
	metadataKey := path.Join(s.config.EtcdS3Folder, metadataDir, basename)

	sf := &snapshotFile{
		Name:     basename,
		Location: fmt.Sprintf("s3://%s/%s", s.config.EtcdS3BucketName, snapshotKey),
		NodeName: "s3",
		CreatedAt: &metav1.Time{
			Time: now,
		},
		S3: &s3Config{
			Endpoint:      s.config.EtcdS3Endpoint,
			EndpointCA:    s.config.EtcdS3EndpointCA,
			SkipSSLVerify: s.config.EtcdS3SkipSSLVerify,
			Bucket:        s.config.EtcdS3BucketName,
			Region:        s.config.EtcdS3Region,
			Folder:        s.config.EtcdS3Folder,
			Insecure:      s.config.EtcdS3Insecure,
		},
		Compressed:     strings.HasSuffix(snapshot, compressedExtension),
		metadataSource: extraMetadata,
		nodeSource:     s.nodeName,
	}

	logrus.Infof("Uploading snapshot to s3://%s/%s", s.config.EtcdS3BucketName, snapshotKey)
	uploadInfo, err := s.uploadSnapshot(ctx, snapshotKey, snapshot)
	if err != nil {
		sf.Status = failedSnapshotStatus
		sf.Message = base64.StdEncoding.EncodeToString([]byte(err.Error()))
	} else {
		sf.Status = successfulSnapshotStatus
		sf.Size = uploadInfo.Size
		sf.tokenHash = s.tokenHash
	}
	if _, err := s.uploadSnapshotMetadata(ctx, metadataKey, metadata); err != nil {
		logrus.Warnf("Failed to upload snapshot metadata to S3: %v", err)
	} else {
		logrus.Infof("Uploaded snapshot metadata s3://%s/%s", s.config.EtcdS3BucketName, metadataKey)
	}
	return sf, err
}

// uploadSnapshot uploads the snapshot file to S3 using the minio API.
func (s *S3) uploadSnapshot(ctx context.Context, key, path string) (info minio.UploadInfo, err error) {
	opts := minio.PutObjectOptions{
		NumThreads: 2,
		UserMetadata: map[string]string{
			clusterIDKey: s.clusterID,
			nodeNameKey:  s.nodeName,
			tokenHashKey: s.tokenHash,
		},
	}
	if strings.HasSuffix(key, compressedExtension) {
		opts.ContentType = "application/zip"
	} else {
		opts.ContentType = "application/octet-stream"
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()

	return s.client.FPutObject(ctx, s.config.EtcdS3BucketName, key, path, opts)
}

// uploadSnapshotMetadata marshals and uploads the snapshot metadata to S3 using the minio API.
// The upload is silently skipped if no extra metadata is provided.
func (s *S3) uploadSnapshotMetadata(ctx context.Context, key, path string) (info minio.UploadInfo, err error) {
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
			clusterIDKey: s.clusterID,
			nodeNameKey:  s.nodeName,
		},
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()
	return s.client.FPutObject(ctx, s.config.EtcdS3BucketName, key, path, opts)
}

// Download downloads the given snapshot from the configured S3
// compatible backend.
func (s *S3) Download(ctx context.Context) error {
	snapshotKey := path.Join(s.config.EtcdS3Folder, s.config.ClusterResetRestorePath)
	metadataKey := path.Join(s.config.EtcdS3Folder, metadataDir, s.config.ClusterResetRestorePath)
	snapshotDir, err := snapshotDir(s.config, true)
	if err != nil {
		return errors.Wrap(err, "failed to get the snapshot dir")
	}
	snapshotFile := filepath.Join(snapshotDir, s.config.ClusterResetRestorePath)
	metadataFile := filepath.Join(snapshotDir, "..", metadataDir, s.config.ClusterResetRestorePath)

	if err := s.downloadSnapshot(ctx, snapshotKey, snapshotFile); err != nil {
		return err
	}
	if err := s.downloadSnapshotMetadata(ctx, metadataKey, metadataFile); err != nil {
		return err
	}

	s.config.ClusterResetRestorePath = snapshotFile
	return nil
}

// downloadSnapshot downloads the snapshot file from S3 using the minio API.
func (s *S3) downloadSnapshot(ctx context.Context, key, file string) error {
	logrus.Debugf("Downloading snapshot from s3://%s/%s", s.config.EtcdS3BucketName, key)
	ctx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()
	defer os.Chmod(file, 0600)
	return s.client.FGetObject(ctx, s.config.EtcdS3BucketName, key, file, minio.GetObjectOptions{})
}

// downloadSnapshotMetadata downloads the snapshot metadata file from S3 using the minio API.
// No error is returned if the metadata file does not exist, as it is optional.
func (s *S3) downloadSnapshotMetadata(ctx context.Context, key, file string) error {
	logrus.Debugf("Downloading snapshot metadata from s3://%s/%s", s.config.EtcdS3BucketName, key)
	ctx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()
	defer os.Chmod(file, 0600)
	err := s.client.FGetObject(ctx, s.config.EtcdS3BucketName, key, file, minio.GetObjectOptions{})
	if resp := minio.ToErrorResponse(err); resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

// snapshotPrefix returns the prefix used in the
// naming of the snapshots.
func (s *S3) snapshotPrefix() string {
	return path.Join(s.config.EtcdS3Folder, s.config.EtcdSnapshotName)
}

// snapshotRetention prunes snapshots in the configured S3 compatible backend for this specific node.
// Returns a list of pruned snapshot names.
func (s *S3) snapshotRetention(ctx context.Context) ([]string, error) {
	if s.config.EtcdSnapshotRetention < 1 {
		return nil, nil
	}
	logrus.Infof("Applying snapshot retention=%d to snapshots stored in s3://%s/%s", s.config.EtcdSnapshotRetention, s.config.EtcdS3BucketName, s.snapshotPrefix())

	var snapshotFiles []minio.ObjectInfo

	toCtx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()

	opts := minio.ListObjectsOptions{
		Prefix:    s.snapshotPrefix(),
		Recursive: true,
	}
	for info := range s.client.ListObjects(toCtx, s.config.EtcdS3BucketName, opts) {
		if info.Err != nil {
			return nil, info.Err
		}

		// skip metadata
		if path.Base(path.Dir(info.Key)) == metadataDir {
			continue
		}

		snapshotFiles = append(snapshotFiles, info)
	}

	if len(snapshotFiles) <= s.config.EtcdSnapshotRetention {
		return nil, nil
	}

	// sort newest-first so we can prune entries past the retention count
	sort.Slice(snapshotFiles, func(i, j int) bool {
		return snapshotFiles[j].LastModified.Before(snapshotFiles[i].LastModified)
	})

	deleted := []string{}
	for _, df := range snapshotFiles[s.config.EtcdSnapshotRetention:] {
		logrus.Infof("Removing S3 snapshot: s3://%s/%s", s.config.EtcdS3BucketName, df.Key)

		key := path.Base(df.Key)
		if err := s.deleteSnapshot(ctx, key); err != nil {
			return deleted, err
		}
		deleted = append(deleted, key)
	}

	return deleted, nil
}

func (s *S3) deleteSnapshot(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()

	key = path.Join(s.config.EtcdS3Folder, key)
	err := s.client.RemoveObject(ctx, s.config.EtcdS3BucketName, key, minio.RemoveObjectOptions{})
	if err == nil || isNotExist(err) {
		metadataKey := path.Join(path.Dir(key), metadataDir, path.Base(key))
		if merr := s.client.RemoveObject(ctx, s.config.EtcdS3BucketName, metadataKey, minio.RemoveObjectOptions{}); merr != nil && !isNotExist(merr) {
			err = merr
		}
	}

	return err
}

// listSnapshots provides a list of currently stored
// snapshots in S3 along with their relevant
// metadata.
func (s *S3) listSnapshots(ctx context.Context) (map[string]snapshotFile, error) {
	snapshots := map[string]snapshotFile{}
	metadatas := []string{}
	ctx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()

	opts := minio.ListObjectsOptions{
		Prefix:    s.config.EtcdS3Folder,
		Recursive: true,
	}

	objects := s.client.ListObjects(ctx, s.config.EtcdS3BucketName, opts)

	for obj := range objects {
		if obj.Err != nil {
			return nil, obj.Err
		}
		if obj.Size == 0 {
			continue
		}

		if o, err := s.client.StatObject(ctx, s.config.EtcdS3BucketName, obj.Key, minio.StatObjectOptions{}); err != nil {
			logrus.Warnf("Failed to get object metadata: %v", err)
		} else {
			obj = o
		}

		filename := path.Base(obj.Key)
		if path.Base(path.Dir(obj.Key)) == metadataDir {
			metadatas = append(metadatas, obj.Key)
			continue
		}

		basename, compressed := strings.CutSuffix(filename, compressedExtension)
		ts, err := strconv.ParseInt(basename[strings.LastIndexByte(basename, '-')+1:], 10, 64)
		if err != nil {
			ts = obj.LastModified.Unix()
		}

		sf := snapshotFile{
			Name:     filename,
			Location: fmt.Sprintf("s3://%s/%s", s.config.EtcdS3BucketName, obj.Key),
			NodeName: "s3",
			CreatedAt: &metav1.Time{
				Time: time.Unix(ts, 0),
			},
			Size: obj.Size,
			S3: &s3Config{
				Endpoint:      s.config.EtcdS3Endpoint,
				EndpointCA:    s.config.EtcdS3EndpointCA,
				SkipSSLVerify: s.config.EtcdS3SkipSSLVerify,
				Bucket:        s.config.EtcdS3BucketName,
				Region:        s.config.EtcdS3Region,
				Folder:        s.config.EtcdS3Folder,
				Insecure:      s.config.EtcdS3Insecure,
			},
			Status:     successfulSnapshotStatus,
			Compressed: compressed,
			nodeSource: obj.UserMetadata[nodeNameKey],
			tokenHash:  obj.UserMetadata[tokenHashKey],
		}
		sfKey := generateSnapshotConfigMapKey(sf)
		snapshots[sfKey] = sf
	}

	for _, metadataKey := range metadatas {
		filename := path.Base(metadataKey)
		sfKey := generateSnapshotConfigMapKey(snapshotFile{Name: filename, NodeName: "s3"})
		if sf, ok := snapshots[sfKey]; ok {
			logrus.Debugf("Loading snapshot metadata from s3://%s/%s", s.config.EtcdS3BucketName, metadataKey)
			if obj, err := s.client.GetObject(ctx, s.config.EtcdS3BucketName, metadataKey, minio.GetObjectOptions{}); err != nil {
				if isNotExist(err) {
					logrus.Debugf("Failed to get snapshot metadata: %v", err)
				} else {
					logrus.Warnf("Failed to get snapshot metadata for %s: %v", filename, err)
				}
			} else {
				if m, err := ioutil.ReadAll(obj); err != nil {
					if isNotExist(err) {
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

func readS3EndpointCA(endpointCA string) ([]byte, error) {
	ca, err := base64.StdEncoding.DecodeString(endpointCA)
	if err != nil {
		return os.ReadFile(endpointCA)
	}
	return ca, nil
}

func setTransportCA(tr http.RoundTripper, endpointCA string, insecureSkipVerify bool) (http.RoundTripper, error) {
	ca, err := readS3EndpointCA(endpointCA)
	if err != nil {
		return tr, err
	}
	if !isValidCertificate(ca) {
		return tr, errors.New("endpoint-ca is not a valid x509 certificate")
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)

	tr.(*http.Transport).TLSClientConfig = &tls.Config{
		RootCAs:            certPool,
		InsecureSkipVerify: insecureSkipVerify,
	}

	return tr, nil
}

// isValidCertificate checks to see if the given
// byte slice is a valid x509 certificate.
func isValidCertificate(c []byte) bool {
	p, _ := pem.Decode(c)
	if p == nil {
		return false
	}
	if _, err := x509.ParseCertificates(p.Bytes); err != nil {
		return false
	}
	return true
}

func bucketLookupType(endpoint string) minio.BucketLookupType {
	if strings.Contains(endpoint, "aliyun") { // backwards compt with RKE1
		return minio.BucketLookupDNS
	}
	return minio.BucketLookupAuto
}
