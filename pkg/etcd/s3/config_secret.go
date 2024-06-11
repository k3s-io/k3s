package s3

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ErrNoConfigSecret = errNoConfigSecret()

type secretError struct {
	err error
}

func (e *secretError) Error() string {
	return fmt.Sprintf("failed to get etcd S3 config secret: %v", e.err)
}

func (e *secretError) Is(target error) bool {
	switch target {
	case ErrNoConfigSecret:
		return true
	}
	return false
}

func errNoConfigSecret() error { return &secretError{} }

func (c *Controller) getConfigFromSecret(secretName string) (*config.EtcdS3, error) {
	if c.core == nil {
		return nil, &secretError{err: util.ErrCoreNotReady}
	}

	secret, err := c.core.V1().Secret().Get(metav1.NamespaceSystem, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, &secretError{err: err}
	}

	etcdS3 := &config.EtcdS3{
		AccessKey: string(secret.Data["etcd-s3-access-key"]),
		Bucket:    string(secret.Data["etcd-s3-bucket"]),
		Endpoint:  defaultEtcdS3.Endpoint,
		Folder:    string(secret.Data["etcd-s3-folder"]),
		Proxy:     string(secret.Data["etcd-s3-proxy"]),
		Region:    defaultEtcdS3.Region,
		SecretKey: string(secret.Data["etcd-s3-secret-key"]),
		Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
	}

	// Set endpoint from secret if set
	if v, ok := secret.Data["etcd-s3-endpoint"]; ok {
		etcdS3.Endpoint = string(v)
	}

	// Set region from secret if set
	if v, ok := secret.Data["etcd-s3-region"]; ok {
		etcdS3.Region = string(v)
	}

	// Set timeout from secret if set
	if v, ok := secret.Data["etcd-s3-timeout"]; ok {
		if duration, err := time.ParseDuration(string(v)); err != nil {
			logrus.Warnf("Failed to parse etcd-s3-timeout value from S3 config secret %s: %v", secretName, err)
		} else {
			etcdS3.Timeout.Duration = duration
		}
	}

	// configure ssl verification, if value can be parsed
	if v, ok := secret.Data["etcd-s3-skip-ssl-verify"]; ok {
		if b, err := strconv.ParseBool(string(v)); err != nil {
			logrus.Warnf("Failed to parse etcd-s3-skip-ssl-verify value from S3 config secret %s: %v", secretName, err)
		} else {
			etcdS3.SkipSSLVerify = b
		}
	}

	// configure insecure http, if value can be parsed
	if v, ok := secret.Data["etcd-s3-insecure"]; ok {
		if b, err := strconv.ParseBool(string(v)); err != nil {
			logrus.Warnf("Failed to parse etcd-s3-insecure value from S3 config secret %s: %v", secretName, err)
		} else {
			etcdS3.Insecure = b
		}
	}

	// encode CA bundles from value, and keys in configmap if one is named
	caBundles := []string{}
	// Add inline CA bundle if set
	if len(secret.Data["etcd-s3-endpoint-ca"]) > 0 {
		caBundles = append(caBundles, base64.StdEncoding.EncodeToString(secret.Data["etcd-s3-endpoint-ca"]))
	}

	// Add CA bundles from named configmap if set
	if caConfigMapName := string(secret.Data["etcd-s3-endpoint-ca-name"]); caConfigMapName != "" {
		configMap, err := c.core.V1().ConfigMap().Get(metav1.NamespaceSystem, caConfigMapName, metav1.GetOptions{})
		if err != nil {
			logrus.Warnf("Failed to get ConfigMap %s for etcd-s3-endpoint-ca-name value from S3 config secret %s: %v", caConfigMapName, secretName, err)
		} else {
			for _, v := range configMap.Data {
				caBundles = append(caBundles, base64.StdEncoding.EncodeToString([]byte(v)))
			}
			for _, v := range configMap.BinaryData {
				caBundles = append(caBundles, base64.StdEncoding.EncodeToString(v))
			}
		}
	}

	// Concatenate all requested CA bundle strings into config var
	etcdS3.EndpointCA = strings.Join(caBundles, " ")
	return etcdS3, nil
}
