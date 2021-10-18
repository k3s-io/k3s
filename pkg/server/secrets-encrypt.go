package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/rancher/k3s/pkg/daemons/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	Start     string = "start"
	Prepare   string = "prepare"
	Rotate    string = "rotate"
	Reencrypt string = "rencrypt"
)

const aescbcKeySize = 32

type EncryptionState struct {
	Stage      string `json:"stage"`
	CurrentKey apiserverconfigv1.Key
}

func GetEncryptionProviders(controlConfig config.Control) ([]apiserverconfigv1.ProviderConfiguration, error) {
	curEncryptionByte, err := ioutil.ReadFile(controlConfig.Runtime.EncryptionConfig)
	if err != nil {
		return nil, err
	}

	curEncryption := apiserverconfigv1.EncryptionConfiguration{}
	if err = json.Unmarshal(curEncryptionByte, &curEncryption); err != nil {
		return nil, err
	}
	return curEncryption.Resources[0].Providers, nil
}

func GetEncryptionKeys(controlConfig config.Control) ([]apiserverconfigv1.Key, error) {

	providers, err := GetEncryptionProviders(controlConfig)
	if err != nil {
		return nil, err
	}
	if len(providers) > 2 {
		return nil, fmt.Errorf("more than 2 providers (%d) found in secrets encryption", len(providers))
	}

	var curKeys []apiserverconfigv1.Key
	for _, p := range providers {
		if p.AESCBC != nil {
			curKeys = append(curKeys, p.AESCBC.Keys...)
		}
	}
	return curKeys, nil
}

func AppendNewEncryptionKey(keys *[]apiserverconfigv1.Key) error {

	aescbcKey := make([]byte, aescbcKeySize)
	_, err := rand.Read(aescbcKey)
	if err != nil {
		return err
	}
	encodedKey := base64.StdEncoding.EncodeToString(aescbcKey)

	newKey := []apiserverconfigv1.Key{
		{
			Name:   "aescbckey-" + time.Now().Format(time.RFC3339),
			Secret: encodedKey,
		},
	}
	*keys = append(*keys, newKey...)
	return nil
}

func WriteEncryptionConfig(controlConfig config.Control, keys []apiserverconfigv1.Key, enable bool) error {

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
	return ioutil.WriteFile(controlConfig.Runtime.EncryptionConfig, jsonfile, 0600)
}

func WriteEncryptionState(controlConfig config.Control, stage string, key apiserverconfigv1.Key) error {

	encStatus := EncryptionState{
		Stage:      stage,
		CurrentKey: key,
	}
	jsonfile, err := json.Marshal(encStatus)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(controlConfig.Runtime.EncryptionState, jsonfile, 0600)
}

func GetEncryptionState(controlConfig config.Control) (string, apiserverconfigv1.Key, error) {
	curEncryptionByte, err := ioutil.ReadFile(controlConfig.Runtime.EncryptionState)
	if err != nil {
		return "", apiserverconfigv1.Key{}, err
	}

	curEncryption := EncryptionState{}
	if err = json.Unmarshal(curEncryptionByte, &curEncryption); err != nil {
		return "", apiserverconfigv1.Key{}, err
	}
	return curEncryption.Stage, curEncryption.CurrentKey, nil
}

func GetEncryptionHashAnnotations(ctx context.Context, k8s kubernetes.Interface) (string, error) {
	nodeName := os.Getenv("NODE_NAME")
	// Try hostname
	if nodeName == "" {
		return "", fmt.Errorf("NODE_NAME not found")
	}
	node, err := k8s.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return node.Annotations[EncryptionConfigHashAnnotation], nil
}
