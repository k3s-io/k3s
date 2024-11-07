package secretsencrypt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/k3s-io/k3s/pkg/generated/clientset/versioned/scheme"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"

	"k8s.io/client-go/rest"
)

const (
	EncryptionStart             string  = "start"
	EncryptionPrepare           string  = "prepare"
	EncryptionRotate            string  = "rotate"
	EncryptionRotateKeys        string  = "rotate_keys"
	EncryptionReencryptRequest  string  = "reencrypt_request"
	EncryptionReencryptActive   string  = "reencrypt_active"
	EncryptionReencryptFinished string  = "reencrypt_finished"
	SecretListPageSize          int64   = 20
	SecretQPS                   float32 = 200
	SecretBurst                 int     = 200
	SecretsUpdateErrorEvent     string  = "SecretsUpdateError"
	SecretsProgressEvent        string  = "SecretsProgress"
	SecretsUpdateCompleteEvent  string  = "SecretsUpdateComplete"
)

var EncryptionHashAnnotation = version.Program + ".io/encryption-config-hash"

func GetEncryptionProviders(runtime *config.ControlRuntime) ([]apiserverconfigv1.ProviderConfiguration, error) {
	curEncryptionByte, err := os.ReadFile(runtime.EncryptionConfig)
	if err != nil {
		return nil, err
	}

	curEncryption := apiserverconfigv1.EncryptionConfiguration{}
	if err = json.Unmarshal(curEncryptionByte, &curEncryption); err != nil {
		return nil, err
	}
	return curEncryption.Resources[0].Providers, nil
}

// GetEncryptionKeys returns a list of encryption keys from the current encryption configuration.
// If includeIdentity is true, it will also include a fake key representing the identity provider, which
// is used to determine if encryption is enabled/disabled.
func GetEncryptionKeys(runtime *config.ControlRuntime, includeIdentity bool) ([]apiserverconfigv1.Key, error) {

	providers, err := GetEncryptionProviders(runtime)
	if err != nil {
		return nil, err
	}
	if len(providers) > 2 {
		return nil, fmt.Errorf("more than 2 providers (%d) found in secrets encryption", len(providers))
	}

	var curKeys []apiserverconfigv1.Key
	for _, p := range providers {
		// Since identity doesn't have keys, we make up a fake key to represent it, so we can
		// know that encryption is enabled/disabled in the request.
		if p.Identity != nil && includeIdentity {
			curKeys = append(curKeys, apiserverconfigv1.Key{
				Name:   "identity",
				Secret: "identity",
			})
		}
		if p.AESCBC != nil {
			curKeys = append(curKeys, p.AESCBC.Keys...)
		}
		if p.AESGCM != nil || p.KMS != nil || p.Secretbox != nil {
			return nil, fmt.Errorf("non-standard encryption keys found")
		}
	}
	return curKeys, nil
}

func WriteEncryptionConfig(runtime *config.ControlRuntime, keys []apiserverconfigv1.Key, enable bool) error {

	// Placing the identity provider first disables encryption
	var providers []apiserverconfigv1.ProviderConfiguration
	if enable {
		providers = []apiserverconfigv1.ProviderConfiguration{
			{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: keys,
				},
			},
			{
				Identity: &apiserverconfigv1.IdentityConfiguration{},
			},
		}
	} else {
		providers = []apiserverconfigv1.ProviderConfiguration{
			{
				Identity: &apiserverconfigv1.IdentityConfiguration{},
			},
			{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: keys,
				},
			},
		}
	}

	encConfig := apiserverconfigv1.EncryptionConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "EncryptionConfiguration",
			APIVersion: "apiserver.config.k8s.io/v1",
		},
		Resources: []apiserverconfigv1.ResourceConfiguration{
			{
				Resources: []string{"secrets"},
				Providers: providers,
			},
		},
	}
	jsonfile, err := json.Marshal(encConfig)
	if err != nil {
		return err
	}
	return util.AtomicWrite(runtime.EncryptionConfig, jsonfile, 0600)
}

func GenEncryptionConfigHash(runtime *config.ControlRuntime) (string, error) {
	curEncryptionByte, err := os.ReadFile(runtime.EncryptionConfig)
	if err != nil {
		return "", err
	}
	encryptionConfigHash := sha256.Sum256(curEncryptionByte)
	return hex.EncodeToString(encryptionConfigHash[:]), nil
}

// GenReencryptHash generates a sha256 hash from the existing secrets keys and
// any identity providers plus a new key based on the input arguments.
func GenReencryptHash(runtime *config.ControlRuntime, keyName string) (string, error) {

	keys, err := GetEncryptionKeys(runtime, true)
	if err != nil {
		return "", err
	}
	newKey := apiserverconfigv1.Key{
		Name:   keyName,
		Secret: "12345",
	}
	keys = append(keys, newKey)
	b, err := json.Marshal(keys)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

func getEncryptionHashFile(runtime *config.ControlRuntime) (string, error) {
	curEncryptionByte, err := os.ReadFile(runtime.EncryptionHash)
	if err != nil {
		return "", err
	}
	return string(curEncryptionByte), nil
}

func BootstrapEncryptionHashAnnotation(node *corev1.Node, runtime *config.ControlRuntime) error {
	existingAnn, err := getEncryptionHashFile(runtime)
	if err != nil {
		return err
	}
	node.Annotations[EncryptionHashAnnotation] = existingAnn
	return nil
}

// WriteEncryptionHashAnnotation writes the encryption hash to the node annotation and optionally to a file.
// The file is used to track the last stage of the reencryption process.
func WriteEncryptionHashAnnotation(runtime *config.ControlRuntime, node *corev1.Node, skipFile bool, stage string) error {
	encryptionConfigHash, err := GenEncryptionConfigHash(runtime)
	if err != nil {
		return err
	}
	if node.Annotations == nil {
		return fmt.Errorf("node annotations do not exist for %s", node.ObjectMeta.Name)
	}
	ann := stage + "-" + encryptionConfigHash
	node.Annotations[EncryptionHashAnnotation] = ann
	if _, err = runtime.Core.Core().V1().Node().Update(node); err != nil {
		return err
	}
	logrus.Debugf("encryption hash annotation set successfully on node: %s\n", node.ObjectMeta.Name)
	if skipFile {
		return nil
	}
	return os.WriteFile(runtime.EncryptionHash, []byte(ann), 0600)
}

// WaitForEncryptionConfigReload watches the metrics API, polling the latest time the encryption config was reloaded.
func WaitForEncryptionConfigReload(runtime *config.ControlRuntime, reloadSuccesses, reloadTime int64) error {
	var lastFailure string

	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		newReloadTime, newReloadSuccess, err := GetEncryptionConfigMetrics(runtime, false)
		if err != nil {
			return true, err
		}

		if newReloadSuccess <= reloadSuccesses || newReloadTime <= reloadTime {
			lastFailure = fmt.Sprintf("apiserver has not reloaded encryption configuration (reload success: %d/%d, reload timestamp %d/%d)", newReloadSuccess, reloadSuccesses, newReloadTime, reloadTime)
			return false, nil
		}
		logrus.Infof("encryption config reloaded successfully %d times", newReloadSuccess)
		logrus.Debugf("encryption config reloaded at %s", time.Unix(newReloadTime, 0))
		return true, nil
	})
	if err != nil {
		err = fmt.Errorf("%w: %s", err, lastFailure)
	}
	return err
}

// GetEncryptionConfigMetrics fetches the metrics API and returns the last time the encryption config was reloaded
// and the number of times it has been reloaded.
func GetEncryptionConfigMetrics(runtime *config.ControlRuntime, initialMetrics bool) (int64, int64, error) {
	var unixUpdateTime int64
	var reloadSuccessCounter int64
	var lastFailure string
	restConfig, err := clientcmd.BuildConfigFromFlags("", runtime.KubeConfigSupervisor)
	if err != nil {
		return 0, 0, err
	}
	restConfig.GroupVersion = &apiserverconfigv1.SchemeGroupVersion
	restConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return 0, 0, err
	}

	// This is wrapped in a poller because on startup no metrics exist. Its only after the encryption config
	// is modified and the first reload occurs that the metrics are available.
	ctx := context.Background()
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 60*time.Second, true, func(ctx context.Context) (bool, error) {
		data, err := restClient.Get().AbsPath("/metrics").DoRaw(ctx)
		if err != nil {
			return true, err
		}

		reader := bytes.NewReader(data)
		var parser expfmt.TextParser
		mf, err := parser.TextToMetricFamilies(reader)
		if err != nil {
			return true, err
		}
		tsMetric := mf["apiserver_encryption_config_controller_automatic_reload_last_timestamp_seconds"]
		// Potentially multiple metrics with different success/failure labels
		totalMetrics := mf["apiserver_encryption_config_controller_automatic_reloads_total"]

		// First time, no metrics exist, so return zeros
		if tsMetric == nil && totalMetrics == nil && initialMetrics {
			return true, nil
		}

		if tsMetric == nil {
			lastFailure = "encryption config time metric not found"
			return false, nil
		}

		if totalMetrics == nil {
			lastFailure = "encryption config total metric not found"
			return false, nil
		}

		unixUpdateTime = int64(tsMetric.GetMetric()[0].GetGauge().GetValue())
		if time.Now().Unix() < unixUpdateTime {
			return true, fmt.Errorf("encryption reload time is incorrectly ahead of current time")
		}

		for _, totalMetric := range totalMetrics.GetMetric() {
			logrus.Debugf("totalMetric: %+v", totalMetric)
			for _, label := range totalMetric.GetLabel() {
				if label.GetName() == "status" && label.GetValue() == "success" {
					reloadSuccessCounter = int64(totalMetric.GetCounter().GetValue())
				}
			}
		}
		return true, nil
	})

	if err != nil {
		err = fmt.Errorf("%w: %s", err, lastFailure)
	}

	return unixUpdateTime, reloadSuccessCounter, err
}
