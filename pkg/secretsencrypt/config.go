package secretsencrypt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/prometheus/common/expfmt"
	corev1 "k8s.io/api/core/v1"

	"github.com/k3s-io/api/pkg/generated/clientset/versioned/scheme"
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
	AESCBCProvider              string  = "aescbc"
	SecretBoxProvider           string  = "secretbox"
	KeySize                     int     = 32
	SecretListPageSize          int64   = 20
	SecretQPS                   float32 = 200
	SecretBurst                 int     = 200
	SecretsUpdateErrorEvent     string  = "SecretsUpdateError"
	SecretsProgressEvent        string  = "SecretsProgress"
	SecretsUpdateCompleteEvent  string  = "SecretsUpdateComplete"
)

// We support 3 key/provider types: AESCBC, SecretBox, and Identity. The Identity provider is
// represented just as a boolean, which is used to determine if encryption is enabled/disabled.
type EncryptionKeys struct {
	AESCBCKeys []apiserverconfigv1.Key
	SBKeys     []apiserverconfigv1.Key
	Identity   bool
}

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
func GetEncryptionKeys(runtime *config.ControlRuntime) (*EncryptionKeys, error) {

	currentKeys := &EncryptionKeys{}
	providers, err := GetEncryptionProviders(runtime)
	if err != nil {
		return nil, err
	}
	if len(providers) > 3 {
		return nil, fmt.Errorf("more than 3 providers (%d) found in secrets encryption", len(providers))
	}

	for _, p := range providers {
		// Since identity doesn't have keys, we make up a fake key to represent it, so we can
		// know that encryption is enabled/disabled in the request.
		if p.Identity != nil {
			currentKeys.Identity = true
		}
		if p.AESCBC != nil {
			currentKeys.AESCBCKeys = append(currentKeys.AESCBCKeys, p.AESCBC.Keys...)
		}
		if p.Secretbox != nil {
			currentKeys.SBKeys = append(currentKeys.SBKeys, p.Secretbox.Keys...)
		}
		if p.AESGCM != nil || p.KMS != nil {
			return nil, fmt.Errorf("unsupported encryption keys found")
		}
	}
	return currentKeys, nil
}

// WriteEncryptionConfig writes the encryption configuration to the file system.
// The provider arg will be placed first, and is used to encrypt new secrets.
func WriteEncryptionConfig(runtime *config.ControlRuntime, keys *EncryptionKeys, provider string, enable bool) error {

	var providers []apiserverconfigv1.ProviderConfiguration
	var primary apiserverconfigv1.ProviderConfiguration
	var secondary *apiserverconfigv1.ProviderConfiguration
	switch provider {
	case AESCBCProvider:
		primary = apiserverconfigv1.ProviderConfiguration{
			AESCBC: &apiserverconfigv1.AESConfiguration{
				Keys: keys.AESCBCKeys,
			},
		}
		if len(keys.SBKeys) > 0 {
			secondary = &apiserverconfigv1.ProviderConfiguration{
				Secretbox: &apiserverconfigv1.SecretboxConfiguration{
					Keys: keys.SBKeys,
				},
			}
		}
	case SecretBoxProvider:
		primary = apiserverconfigv1.ProviderConfiguration{
			Secretbox: &apiserverconfigv1.SecretboxConfiguration{
				Keys: keys.SBKeys,
			},
		}
		if len(keys.AESCBCKeys) > 0 {
			secondary = &apiserverconfigv1.ProviderConfiguration{
				AESCBC: &apiserverconfigv1.AESConfiguration{
					Keys: keys.AESCBCKeys,
				},
			}
		}
	}
	identity := apiserverconfigv1.ProviderConfiguration{
		Identity: &apiserverconfigv1.IdentityConfiguration{},
	}
	// Placing the identity provider first disables encryption
	if enable && secondary != nil {
		providers = []apiserverconfigv1.ProviderConfiguration{
			primary,
			*secondary,
			identity,
		}
	} else if enable {
		providers = []apiserverconfigv1.ProviderConfiguration{
			primary,
			identity,
		}
	} else {
		providers = []apiserverconfigv1.ProviderConfiguration{
			identity,
			primary,
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

// WriteIdentityConfig creates an identity-only configuration for clusters that
// previously had no encryption config, effectively disabling encryption, but
// preparing a node for future reencryption.
func WriteIdentityConfig(control *config.Control) error {
	providers := []apiserverconfigv1.ProviderConfiguration{
		{
			Identity: &apiserverconfigv1.IdentityConfiguration{},
		},
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
	if control.Runtime.EncryptionConfig == "" {
		control.Runtime.EncryptionConfig = filepath.Join(control.DataDir, "cred", "encryption-config.json")
	}
	logrus.Info("Enabling secrets encryption with identity provider, restart with secrets-encryption")
	return util.AtomicWrite(control.Runtime.EncryptionConfig, jsonfile, 0600)
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

	// To retain compatibility with the older encryption hash format,
	// we contruct the hash as: aescbc + secretbox + identity + newkey
	currentKeys, err := GetEncryptionKeys(runtime)
	if err != nil {
		return "", err
	}
	keys := currentKeys.AESCBCKeys
	keys = append(keys, currentKeys.SBKeys...)
	if currentKeys.Identity {
		keys = append(keys, apiserverconfigv1.Key{
			Name:   "identity",
			Secret: "identity",
		})
	}
	keys = append(keys, apiserverconfigv1.Key{
		Name:   keyName,
		Secret: "12345",
	})
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

func BootstrapEncryptionHashAnnotation(ctx context.Context, runtime *config.ControlRuntime, nodeName string) error {
	existingAnn, err := getEncryptionHashFile(runtime)
	if err != nil {
		return err
	}
	patch := util.NewPatchList()
	patcher := util.NewPatcher[*corev1.Node](runtime.Core.Core().V1().Node())
	patch.Add(existingAnn, "metadata", "annotations", EncryptionHashAnnotation)

	_, err = patcher.Patch(ctx, patch, nodeName)
	return err
}

// WriteEncryptionHashAnnotation writes the encryption hash to the node annotation and optionally to a file.
// The file is used to track the last stage of the reencryption process.
func WriteEncryptionHashAnnotation(ctx context.Context, runtime *config.ControlRuntime, nodeName string, skipFile bool, stage string) error {
	encryptionConfigHash, err := GenEncryptionConfigHash(runtime)
	if err != nil {
		return err
	}
	ann := stage + "-" + encryptionConfigHash

	patch := util.NewPatchList()
	patcher := util.NewPatcher[*corev1.Node](runtime.Core.Core().V1().Node())
	patch.Add(ann, "metadata", "annotations", EncryptionHashAnnotation)
	if _, err = patcher.Patch(ctx, patch, nodeName); err != nil {
		return err
	}
	logrus.Debugf("encryption hash annotation set successfully on node: %s\n", nodeName)
	if skipFile {
		return nil
	}
	return os.WriteFile(runtime.EncryptionHash, []byte(ann), 0600)
}

// WaitForEncryptionConfigReload watches the metrics API, polling the latest time the encryption config was reloaded.
func WaitForEncryptionConfigReload(runtime *config.ControlRuntime, reloadSuccesses, reloadTime int64) error {
	var lastFailure string

	ctx := context.Background()
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 120*time.Second, true, func(ctx context.Context) (bool, error) {
		newReloadTime, newReloadSuccess, err := GetEncryptionConfigMetrics(runtime, false)
		if err != nil {
			return true, err
		}

		if newReloadSuccess <= reloadSuccesses || newReloadTime <= reloadTime {
			lastFailure = fmt.Sprintf("apiserver has not reloaded encryption configuration (reload success: %d/%d, reload timestamp %d/%d)", newReloadSuccess, reloadSuccesses, newReloadTime, reloadTime)
			logrus.Debugf("waiting for apiserver to reload encryption config: %s", lastFailure)
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
	restConfig, err := util.GetRESTConfig(runtime.KubeConfigSupervisor)
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
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 120*time.Second, true, func(ctx context.Context) (bool, error) {
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
