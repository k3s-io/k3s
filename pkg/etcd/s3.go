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
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
)

// s3 maintains state for S3 functionality.
type s3 struct {
	config *config.Control
	client *minio.Client
}

// newS3 creates a new value of type s3 pointer with a
// copy of the config.Control pointer and initializes
// a new Minio client.
func newS3(config *config.Control) (*s3, error) {
	tr := http.DefaultTransport
	if config.EtcdS3EndpointCA != "" {
		trCA, err := setTransportCA(tr, config.EtcdS3EndpointCA, config.EtcdS3SkipSSLVerify)
		if err != nil {
			return nil, err
		}
		tr = trCA
	}

	var creds *credentials.Credentials
	if len(config.EtcdS3AccessKey) == 0 && len(config.EtcdS3SecretKey) == 0 {
		creds = credentials.NewIAM("") // for running on ec2 instance
	} else {
		creds = credentials.NewStaticV4(config.EtcdS3AccessKey, config.EtcdS3SecretKey, "")
	}

	opt := minio.Options{
		Creds:        creds,
		Secure:       true,
		Region:       config.EtcdS3Region,
		Transport:    tr,
		BucketLookup: bucketLookupType(config.EtcdS3Endpoint),
	}
	c, err := minio.New(config.EtcdS3Endpoint, &opt)
	if err != nil {
		return nil, err
	}

	return &s3{
		config: config,
		client: c,
	}, nil
}

// upload uploads the given snapshot to the configured S3
// compatible backend.
func (s *s3) upload(ctx context.Context, snapshot string) error {
	exists, err := s.client.BucketExists(ctx, s.config.EtcdS3BucketName)
	if err != nil {
		return nil
	}
	if !exists {
		return fmt.Errorf("the given bucket: %s does not exist", s.config.EtcdS3BucketName)
	}

	// Upload the zip file with FPutObject
	opts := minio.PutObjectOptions{
		ContentType: "application/zip",
	}
	const retries = 3
	for retries := 0; retries <= retries; retries++ {
		if _, err := s.client.FPutObject(ctx, s.config.EtcdS3BucketName, filepath.Base(snapshot), snapshot, opts); err != nil {
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
	if endpoint == "" {
		return minio.BucketLookupAuto
	}
	if strings.Contains(endpoint, "aliyun") {
		return minio.BucketLookupDNS
	}
	return minio.BucketLookupAuto
}
