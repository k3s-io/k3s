package server

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
)

const (
	EncryptionStart     string = "start"
	EncryptionPrepare   string = "prepare"
	EncryptionRotate    string = "rotate"
	EncryptionReencrypt string = "reencrypt"
)

const aescbcKeySize = 32

type EncryptionState struct {
	Stage      string `json:"stage"`
	CurrentKey apiserverconfigv1.Key
}

type EncryptionRequest struct {
	Stage  string `json:"stage,omitempty"`
	Toggle bool   `json:"toggle,omitempty"`
	Force  bool   `json:"force"`
}

func getEncryptionRequest(req *http.Request) (EncryptionRequest, error) {
	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return EncryptionRequest{}, err
	}
	result := EncryptionRequest{}
	err = json.Unmarshal(b, &result)
	return result, err
}

func encryptionStatusHandler(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		status, err := encryptionStatus(server)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
			return
		}
		resp.Write([]byte(status))
	})
}

func encryptionStatus(server *config.Control) (string, error) {
	providers, err := getEncryptionProviders(server)
	if os.IsNotExist(err) {
		return "Encryption Status: Disabled, no configuration file found", nil
	} else if err != nil {
		return "", err
	}
	var statusOutput string
	if providers[1].Identity != nil && providers[0].AESCBC != nil {
		statusOutput += "Encryption Status: Enabled\n"
	} else if providers[0].Identity != nil && providers[1].AESCBC != nil || !server.EncryptSecrets {
		statusOutput += "Encryption Status: Disabled"
	}

	stage, _, err := getEncryptionState(server)
	if err != nil {
		return "", err
	}
	statusOutput += fmt.Sprintln("Current Rotation Stage:", stage)

	if err := verifyEncryptionHashAnnotation(server.Runtime.Core.Core()); err != nil {
		statusOutput += fmt.Sprintf("Server Encryption Hashes: %s\n", err.Error())
	} else {
		statusOutput += fmt.Sprintln("Server Encryption Hashes: All hashes match")
	}
	var tabBuffer bytes.Buffer
	w := tabwriter.NewWriter(&tabBuffer, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Key Type\tName\tSecret\n")
	fmt.Fprintf(w, "--------\t----\t------\n")
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

func encryptionToggleHandler(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		if req.Method != http.MethodPut {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		encryptReq, err := getEncryptionRequest(req)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
			return
		}

		if err := encryptionToggle(server, encryptReq.Toggle); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
			return
		}
		resp.WriteHeader(http.StatusOK)
	})
}

func encryptionToggle(server *config.Control, enable bool) error {
	providers, err := getEncryptionProviders(server)
	if err != nil {
		return err
	}
	if len(providers) > 2 {
		return fmt.Errorf("more than 2 providers (%d) found in secrets encryption", len(providers))
	}
	curKeys, err := getEncryptionKeys(server)
	if err != nil {
		return err
	}
	if providers[1].Identity != nil && providers[0].AESCBC != nil && !enable {
		logrus.Infoln("Disabling secrets encryption")
		if err := writeEncryptionConfig(server, curKeys, enable); err != nil {
			return err
		}
	} else if !enable {
		logrus.Infoln("Secrets encryption already disabled")
		return nil
	} else if providers[0].Identity != nil && providers[1].AESCBC != nil && enable {
		logrus.Infoln("Enabling secrets encryption")
		if err := writeEncryptionConfig(server, curKeys, enable); err != nil {
			return err
		}
	} else if enable {
		logrus.Infoln("Secrets encryption already enabled")
		return nil
	} else {
		return fmt.Errorf("unable to enable/disable secrets encryption, unknown configuration")
	}
	return updateSecrets(server.Runtime.Core.Core())
}

func encryptionStageHandler(server *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if req.TLS == nil {
			resp.WriteHeader(http.StatusNotFound)
			return
		}
		if req.Method != http.MethodPut {
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		encryptReq, err := getEncryptionRequest(req)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
			return
		}
		switch encryptReq.Stage {
		case EncryptionPrepare:
			err = encryptionPrepare(server, encryptReq.Force)
		case EncryptionRotate:
			err = encryptionRotate(server, encryptReq.Force)
		case EncryptionReencrypt:
			err = encryptionReencrypt(server, encryptReq.Force)
		default:
			err = fmt.Errorf("unknown stage requested")
		}
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(err.Error()))
			return
		}
		resp.WriteHeader(http.StatusOK)
	})
}

func encryptionPrepare(server *config.Control, force bool) error {
	stage, key, err := getEncryptionState(server)
	if err != nil {
		return err
	} else if !force && (stage != EncryptionStart && stage != EncryptionReencrypt) {
		return fmt.Errorf("error, incorrect stage %s found with key %s", stage, key.Name)
	}

	if err := verifyEncryptionHashAnnotation(server.Runtime.Core.Core()); err != nil {
		return err
	}

	curKeys, err := getEncryptionKeys(server)
	if err != nil {
		return err
	}

	if err := AppendNewEncryptionKey(&curKeys); err != nil {
		return err
	}
	logrus.Infoln("Adding secrets-encryption key: ", curKeys[len(curKeys)-1])

	if err := writeEncryptionConfig(server, curKeys, true); err != nil {
		return err
	}
	if err := writeEncryptionState(server, EncryptionPrepare, curKeys[0]); err != nil {
		return err
	}
	return writeEncryptionHashAnnotation(server, server.Runtime.Core.Core())
}

func encryptionRotate(server *config.Control, force bool) error {
	stage, key, err := getEncryptionState(server)
	if err != nil {
		return err
	} else if !force && stage != EncryptionPrepare {
		return fmt.Errorf("error, incorrect stage %s found with key %s", stage, key.Name)
	}

	if err := verifyEncryptionHashAnnotation(server.Runtime.Core.Core()); err != nil {
		return err
	}

	curKeys, err := getEncryptionKeys(server)
	if err != nil {
		return err
	}

	// Right rotate elements
	rotatedKeys := append(curKeys[len(curKeys)-1:], curKeys[:len(curKeys)-1]...)

	if err = writeEncryptionConfig(server, rotatedKeys, true); err != nil {
		return err
	}
	if err := writeEncryptionState(server, EncryptionRotate, curKeys[0]); err != nil {
		return err
	}
	logrus.Infoln("Encryption keys right rotated")
	return writeEncryptionHashAnnotation(server, server.Runtime.Core.Core())
}

func encryptionReencrypt(server *config.Control, force bool) error {
	stage, key, err := getEncryptionState(server)
	if err != nil {
		return err
	} else if !force && stage != EncryptionRotate {
		return fmt.Errorf("error, incorrect stage %s found with key %s", stage, key.Name)
	}
	if err := verifyEncryptionHashAnnotation(server.Runtime.Core.Core()); err != nil {
		return err
	}

	updateSecrets(server.Runtime.Core.Core())

	// Remove last key
	curKeys, err := getEncryptionKeys(server)
	if err != nil {
		return err
	}

	curKeys = curKeys[:len(curKeys)-1]
	if err = writeEncryptionConfig(server, curKeys, true); err != nil {
		return err
	}

	if err := writeEncryptionState(server, EncryptionReencrypt, curKeys[0]); err != nil {
		return err
	}
	logrus.Infoln("Removed key: ", curKeys[len(curKeys)-1])
	return writeEncryptionHashAnnotation(server, server.Runtime.Core.Core())
}

func updateSecrets(core core.Interface) error {
	secrets, err := core.V1().Secret().List("", metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, s := range secrets.Items {
		_, err := core.V1().Secret().Update(&s)
		if err != nil {
			return err
		}
	}
	logrus.Infof("Updated %d secrets with new key\n", len(secrets.Items))
	return nil
}

func getEncryptionProviders(controlConfig *config.Control) ([]apiserverconfigv1.ProviderConfiguration, error) {
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

func getEncryptionKeys(controlConfig *config.Control) ([]apiserverconfigv1.Key, error) {

	providers, err := getEncryptionProviders(controlConfig)
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

func writeEncryptionConfig(controlConfig *config.Control, keys []apiserverconfigv1.Key, enable bool) error {

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

func writeEncryptionState(controlConfig *config.Control, stage string, key apiserverconfigv1.Key) error {

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

func getEncryptionState(controlConfig *config.Control) (string, apiserverconfigv1.Key, error) {
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

func verifyEncryptionHashAnnotation(core core.Interface) error {
	nodes, err := core.V1().Node().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	var serverNodes []corev1.Node
	for _, node := range nodes.Items {
		if v, ok := node.Labels[ControlPlaneRoleLabelKey]; ok && v == "true" {
			serverNodes = append(serverNodes, node)
		}
	}

	var firstHash string
	var firstNodeName string
	first := true
	for _, node := range serverNodes {
		hash, ok := node.Annotations[encryptionHashAnnotation]
		if ok && first {
			firstHash = hash
			first = false
			firstNodeName = node.ObjectMeta.Name
		} else if ok && hash != firstHash {
			return fmt.Errorf("hash does not match between %s and %s", firstNodeName, node.ObjectMeta.Name)
		}
	}
	return nil
}

func writeEncryptionHashAnnotation(server *config.Control, core core.Interface) error {
	curEncryptionByte, err := ioutil.ReadFile(server.Runtime.EncryptionConfig)
	if err != nil {
		return err
	}
	encryptionConfigHash := sha256.Sum256(curEncryptionByte)
	nodeName := os.Getenv("NODE_NAME")
	node, err := core.V1().Node().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if node.Annotations == nil {
		return fmt.Errorf("node annotations do not exist for %s", nodeName)
	}
	node.Annotations[encryptionHashAnnotation] = hex.EncodeToString(encryptionConfigHash[:])
	if _, err = core.V1().Node().Update(node); err != nil {
		return err
	}
	logrus.Debugf("encryption hash annotation set successfully on node: %s\n", nodeName)
	return nil
}
