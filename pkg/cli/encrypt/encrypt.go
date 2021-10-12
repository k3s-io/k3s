package encrypt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"text/tabwriter"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/server"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/kubernetes"
)

const aescbcKeySize = 32

func pp(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
func commandPrep(cfg *cmds.Server) (config.Control, error) {
	var configControl config.Control
	var err error
	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	gspt.SetProcTitle(os.Args[0] + " encrypt")

	configControl.DataDir, err = server.ResolveDataDir(cfg.DataDir)
	if err != nil {
		return configControl, err
	}
	configControl.Runtime = &config.ControlRuntime{}
	configControl.EncryptSecrets = cfg.EncryptSecrets
	fmt.Println("HELP ", cfg.EncryptSecrets)
	configControl.Runtime.EncryptionConfig = filepath.Join(configControl.DataDir, "cred", "encryption-config.json")
	configControl.Runtime.KubeConfigAdmin = filepath.Join(configControl.DataDir, "cred", "admin.kubeconfig")

	return configControl, nil
}

func Run(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	configControl, err := commandPrep(&cmds.ServerConfig)
	if err != nil {
		return err
	}
	return encryptionStatus(configControl)
}

func Status(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	configControl, err := commandPrep(&cmds.ServerConfig)
	if err != nil {
		return err
	}
	return encryptionStatus(configControl)
}

func Prepare(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	configControl, err := commandPrep(&cmds.ServerConfig)
	if err != nil {
		return err
	}

	// If hash exists, check that they don't match to prevent prepare running twice
	hashFile := filepath.Join(configControl.DataDir, "cred", "encryption-config.sha256")
	if existingHash, err := ioutil.ReadFile(hashFile); err == nil {
		currentHash := getEncryptionHash(configControl)
		logrus.Debugf("Existing hash: %x, current hash: %x", existingHash, currentHash)
		if reflect.DeepEqual(existingHash, currentHash[:]) {
			fmt.Println("Existing prepare operation detected, aborting prepare")
			return nil
		}
	}

	providers, err := getEncryptionProviders(configControl)
	if err != nil {
		return err
	}
	if len(providers) > 2 {
		return fmt.Errorf("more than 2 providers (%d) found in secrets encryption", len(providers))
	}

	var curKeys []apiserverconfigv1.Key
	for _, p := range providers {
		if p.AESCBC != nil {
			curKeys = append(curKeys, p.AESCBC.Keys...)
		}
	}

	appendNewEncryptionKey(&curKeys)
	fmt.Println("Adding key: ", curKeys[len(curKeys)-1])

	return writeEncryptionConfigAndHash(configControl, hashFile, curKeys, true)
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	configControl, err := commandPrep(&cmds.ServerConfig)
	if err != nil {
		return err
	}

	providers, err := getEncryptionProviders(configControl)
	if err != nil {
		return err
	}
	if len(providers) > 2 {
		return fmt.Errorf("more than 2 providers (%d) found in secrets encryption", len(providers))
	}

	var curKeys []apiserverconfigv1.Key
	for _, p := range providers {
		if p.AESCBC != nil {
			curKeys = append(curKeys, p.AESCBC.Keys...)
		}
	}
	fmt.Println(curKeys)
	// Right rotate elements
	var rotatedKeys []apiserverconfigv1.Key
	rotatedKeys = append(curKeys[len(curKeys)-1:], curKeys[:len(curKeys)-1]...)
	fmt.Println(rotatedKeys)
	return nil
}

func Reencrypt(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return nil
}

func readSecrets(ctx context.Context, configControl config.Control, k8s kubernetes.Interface) error {
	secrets, err := k8s.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	fmt.Println(pp(secrets))
	return nil
}

func getEncryptionProviders(configControl config.Control) ([]apiserverconfigv1.ProviderConfiguration, error) {
	curEncryptionByte, err := ioutil.ReadFile(configControl.Runtime.EncryptionConfig)
	if err != nil {
		return nil, err
	}

	curEncryption := apiserverconfigv1.EncryptionConfiguration{}
	if err = json.Unmarshal(curEncryptionByte, &curEncryption); err != nil {
		return nil, err
	}
	return curEncryption.Resources[0].Providers, nil
}

func appendNewEncryptionKey(keys *[]apiserverconfigv1.Key) error {

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

func writeEncryptionConfigAndHash(configControl config.Control, hashFile string, keys []apiserverconfigv1.Key, enable bool) error {

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
	if err = ioutil.WriteFile(configControl.Runtime.EncryptionConfig, jsonfile, 0600); err != nil {
		return err
	}
	encryptionHash := getEncryptionHash(configControl)
	return ioutil.WriteFile(hashFile, encryptionHash[:], 0600)
}

func getEncryptionHash(configControl config.Control) [32]byte {
	curEncryptionByte, err := ioutil.ReadFile(configControl.Runtime.EncryptionConfig)
	if err != nil {
		logrus.Fatal("no secrets encryption file found")
	}
	return sha256.Sum256(curEncryptionByte)
}

func encryptionStatus(configControl config.Control) error {
	if !configControl.EncryptSecrets {
		fmt.Println("Encryption Status: Disabled")
		// return nil
	}
	fmt.Println("Encryption Status: Enabled")

	providers, err := getEncryptionProviders(configControl)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
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

	return w.Flush()
}
