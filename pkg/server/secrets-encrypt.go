package server

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
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

func encryptionPrepareHandler(server *config.Control, force bool) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		if req.Method != http.MethodPut {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		if err := encryptionPrepare(server, force); err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			resp.Write([]byte(err.Error()))
			return
		}
		resp.WriteHeader(http.StatusOK)
	})
}

func encryptionPrepare(server *config.Control, force bool) error {
	stage, key, err := GetEncryptionState(*server)
	if err != nil {
		return err
	} else if !force && (stage != Start && stage != Reencrypt) {
		return fmt.Errorf("error, incorrect stage %s found with key %s", stage, key.Name)
	}

	curKeys, err := GetEncryptionKeys(*server)
	if err != nil {
		return err
	}

	if err := AppendNewEncryptionKey(&curKeys); err != nil {
		return err
	}
	logrus.Infoln("Adding secrets-encryption key: ", curKeys[len(curKeys)-1])

	if err := WriteEncryptionConfig(*server, curKeys, true); err != nil {
		return err
	}
	return WriteEncryptionState(*server, Prepare, curKeys[0])
}

func encryptionStatusHandler(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		status, err := encryptionStatus(server)
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			resp.Write([]byte(err.Error()))
			return
		}
		resp.Write([]byte(status))
	})
}

func encryptionStatus(controlConfig *config.Control) (string, error) {
	providers, err := GetEncryptionProviders(*controlConfig)
	if os.IsNotExist(err) {
		return "Encryption Status: Disabled, no configuration file found", nil
	} else if err != nil {
		return "", err
	}
	var statusOutput string
	if providers[1].Identity != nil && providers[0].AESCBC != nil {
		statusOutput += "Encryption Status: Enabled\n"
	} else if providers[0].Identity != nil && providers[1].AESCBC != nil || !controlConfig.EncryptSecrets {
		statusOutput += "Encryption Status: Disabled"
	}

	stage, _, err := GetEncryptionState(*controlConfig)
	if err != nil {
		return "", err
	}
	statusOutput += fmt.Sprintln("Current Rotation Stage:", stage)

	var tabBuffer bytes.Buffer
	w := tabwriter.NewWriter(&tabBuffer, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Key Type\tName\tSecret\n")

	for _, p := range providers {
		if p.AESCBC != nil {
			for _, aesKey := range p.AESCBC.Keys {
				fmt.Fprintf(w, "%s\t%s\t%s\n", "AES-CBC", aesKey.Name, aesKey.Secret)
			}
		}
		if p.Identity != nil {
			fmt.Fprintf(w, "Identity\tidentity\tN/A\n")
		}
	}
	w.Flush()

	return statusOutput + tabBuffer.String(), nil
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
