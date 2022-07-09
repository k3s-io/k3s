package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// S3 maintains state for S3 functionality.
type S3 struct {
	config *config.Control
	client *minio.Client
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
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("bucket: %s does not exist", config.EtcdS3BucketName)
	}
	logrus.Infof("S3 bucket %s exists", config.EtcdS3BucketName)

	return &S3{
		config: config,
		client: c,
	}, nil
}

// upload uploads the given snapshot to the configured S3
// compatible backend.
func (s *S3) upload(ctx context.Context, snapshot, extraMetadata string, now time.Time) (*snapshotFile, error) {
	logrus.Infof("Uploading snapshot %s to S3", snapshot)
	basename := filepath.Base(snapshot)
	var snapshotFileName string
	var sf snapshotFile
	if s.config.EtcdS3Folder != "" {
		snapshotFileName = filepath.Join(s.config.EtcdS3Folder, basename)
	} else {
		snapshotFileName = basename
	}

	toCtx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()
	opts := minio.PutObjectOptions{NumThreads: 2}
	if strings.HasSuffix(snapshot, compressedExtension) {
		opts.ContentType = "application/zip"
	} else {
		opts.ContentType = "application/octet-stream"
	}
	uploadInfo, err := s.client.FPutObject(toCtx, s.config.EtcdS3BucketName, snapshotFileName, snapshot, opts)
	if err != nil {
		sf = snapshotFile{
			Name:     filepath.Base(uploadInfo.Key),
			Metadata: extraMetadata,
			NodeName: "s3",
			CreatedAt: &metav1.Time{
				Time: now,
			},
			Message: base64.StdEncoding.EncodeToString([]byte(err.Error())),
			Size:    0,
			Status:  failedSnapshotStatus,
			S3: &s3Config{
				Endpoint:      s.config.EtcdS3Endpoint,
				EndpointCA:    s.config.EtcdS3EndpointCA,
				SkipSSLVerify: s.config.EtcdS3SkipSSLVerify,
				Bucket:        s.config.EtcdS3BucketName,
				Region:        s.config.EtcdS3Region,
				Folder:        s.config.EtcdS3Folder,
				Insecure:      s.config.EtcdS3Insecure,
			},
		}
		logrus.Errorf("Error received during snapshot upload to S3: %s", err)
	} else {
		ca, err := time.Parse(time.RFC3339, uploadInfo.LastModified.Format(time.RFC3339))
		if err != nil {
			return nil, err
		}

		sf = snapshotFile{
			Name:     filepath.Base(uploadInfo.Key),
			Metadata: extraMetadata,
			NodeName: "s3",
			CreatedAt: &metav1.Time{
				Time: ca,
			},
			Size:   uploadInfo.Size,
			Status: successfulSnapshotStatus,
			S3: &s3Config{
				Endpoint:      s.config.EtcdS3Endpoint,
				EndpointCA:    s.config.EtcdS3EndpointCA,
				SkipSSLVerify: s.config.EtcdS3SkipSSLVerify,
				Bucket:        s.config.EtcdS3BucketName,
				Region:        s.config.EtcdS3Region,
				Folder:        s.config.EtcdS3Folder,
				Insecure:      s.config.EtcdS3Insecure,
			},
		}
	}
	return &sf, nil
}

// download downloads the given snapshot from the configured S3
// compatible backend.
func (s *S3) Download(ctx context.Context) error {
	var remotePath string
	if s.config.EtcdS3Folder != "" {
		remotePath = filepath.Join(s.config.EtcdS3Folder, s.config.ClusterResetRestorePath)
	} else {
		remotePath = s.config.ClusterResetRestorePath
	}

	logrus.Debugf("retrieving snapshot: %s", remotePath)
	toCtx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()

	r, err := s.client.GetObject(toCtx, s.config.EtcdS3BucketName, remotePath, minio.GetObjectOptions{})
	if err != nil {
		return nil
	}
	defer r.Close()

	snapshotDir, err := snapshotDir(s.config, true)
	if err != nil {
		return errors.Wrap(err, "failed to get the snapshot dir")
	}

	fullSnapshotPath := filepath.Join(snapshotDir, s.config.ClusterResetRestorePath)
	sf, err := os.Create(fullSnapshotPath)
	if err != nil {
		return err
	}
	defer sf.Close()

	stat, err := r.Stat()
	if err != nil {
		return err
	}

	if _, err := io.CopyN(sf, r, stat.Size); err != nil {
		return err
	}

	s.config.ClusterResetRestorePath = fullSnapshotPath

	return os.Chmod(fullSnapshotPath, 0600)
}

// snapshotPrefix returns the prefix used in the
// naming of the snapshots.
func (s *S3) snapshotPrefix() string {
	nodeName := os.Getenv("NODE_NAME")
	fullSnapshotPrefix := s.config.EtcdSnapshotName + "-" + nodeName
	var prefix string
	if s.config.EtcdS3Folder != "" {
		prefix = filepath.Join(s.config.EtcdS3Folder, fullSnapshotPrefix)
	} else {
		prefix = fullSnapshotPrefix
	}
	return prefix
}

// snapshotRetention prunes snapshots in the configured S3 compatible backend for this specific node.
func (s *S3) snapshotRetention(ctx context.Context) error {
	if s.config.EtcdSnapshotRetention < 1 {
		return nil
	}
	logrus.Infof("Applying snapshot retention policy to snapshots stored in S3: retention: %d, snapshotPrefix: %s", s.config.EtcdSnapshotRetention, s.snapshotPrefix())

	var snapshotFiles []minio.ObjectInfo

	toCtx, cancel := context.WithTimeout(ctx, s.config.EtcdS3Timeout)
	defer cancel()

	loo := minio.ListObjectsOptions{
		Recursive: true,
		Prefix:    s.snapshotPrefix(),
	}
	for info := range s.client.ListObjects(toCtx, s.config.EtcdS3BucketName, loo) {
		if info.Err != nil {
			return info.Err
		}
		snapshotFiles = append(snapshotFiles, info)
	}

	if len(snapshotFiles) <= s.config.EtcdSnapshotRetention {
		return nil
	}

	sort.Slice(snapshotFiles, func(i, j int) bool {
		return snapshotFiles[i].Key < snapshotFiles[j].Key
	})

	delCount := len(snapshotFiles) - s.config.EtcdSnapshotRetention
	for _, df := range snapshotFiles[:delCount] {
		logrus.Infof("Removing S3 snapshot: %s", df.Key)
		if err := s.client.RemoveObject(ctx, s.config.EtcdS3BucketName, df.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func readS3EndpointCA(endpointCA string) ([]byte, error) {
	ca, err := base64.StdEncoding.DecodeString(endpointCA)
	if err != nil {
		return ioutil.ReadFile(endpointCA)
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
