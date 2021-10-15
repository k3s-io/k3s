package secretsencrypt

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
	"strings"
	"text/tabwriter"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/kubernetes"
)

const aescbcKeySize = 32

func pp(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
func commandPrep(app *cli.Context, cfg *cmds.Server) (config.Control, error) {
	var controlConfig config.Control
	var err error
	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	gspt.SetProcTitle(os.Args[0] + " encrypt")

	nodeName := app.String("node-name")
	if nodeName == "" {
		nodeName, err = os.Hostname()
		if err != nil {
			return controlConfig, err
		}
	}

	os.Setenv("NODE_NAME", nodeName)

	controlConfig.DataDir, err = server.ResolveDataDir(cfg.DataDir)
	if err != nil {
		return controlConfig, err
	}
	controlConfig.Runtime = &config.ControlRuntime{}
	controlConfig.EncryptSecrets = cfg.EncryptSecrets
	controlConfig.EncryptForceRotation = cfg.EncryptForceRotation
	controlConfig.Runtime.EncryptionConfig = filepath.Join(controlConfig.DataDir, "cred", "encryption-config.json")
	controlConfig.Runtime.KubeConfigAdmin = filepath.Join(controlConfig.DataDir, "cred", "admin.kubeconfig")

	return controlConfig, nil
}

func Run(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}

	providers, err := getEncryptionProviders(controlConfig)
	if err != nil {
		return err
	}
	if len(providers) > 2 {
		return fmt.Errorf("more than 2 providers (%d) found in secrets encryption", len(providers))
	}
	curKeys, err := getEncryptionKeys(controlConfig)
	if err != nil {
		return err
	}
	if providers[1].Identity != nil && providers[0].AESCBC != nil {
		fmt.Println("Disabling secrets encryption")
		return writeEncryptionConfig(controlConfig, curKeys, false)
	} else if providers[0].Identity != nil && providers[1].AESCBC != nil {
		fmt.Println("Enabling secrets encryption")
		return writeEncryptionConfig(controlConfig, curKeys, true)
	}
	return fmt.Errorf("unable to toggle secrets encryption, unknown configuration")
}

func Status(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}

	return encryptionStatus(controlConfig)
}

func Prepare(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	ctx := signals.SetupSignalHandler(context.Background())
	sc, err := server.NewContext(ctx, controlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	serverNodes, err := getServerNodes(ctx, sc.K8s)
	if err != nil {
		return err
	}
	if err := verifyEncryptionHash(serverNodes); err != nil {
		return err
	}

	curKeys, err := getEncryptionKeys(controlConfig)
	if err != nil {
		return err
	}

	if len(curKeys) > 1 && !askForConfirmation("Warning: More than one key detected! Are you sure you want to add a new key?") {
		return nil
	}

	appendNewEncryptionKey(&curKeys)
	fmt.Println("Adding key: ", curKeys[len(curKeys)-1])

	return writeEncryptionConfig(controlConfig, curKeys, true)
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}

	rotateHashFile := filepath.Join(controlConfig.DataDir, "cred", "encryption-rotate.sha256")
	if _, err := ioutil.ReadFile(rotateHashFile); err == nil && !controlConfig.EncryptForceRotation {
		return fmt.Errorf("existing key rotation detected, aborting rotate operation")
	}

	curKeys, err := getEncryptionKeys(controlConfig)
	if err != nil {
		return err
	}

	// Right rotate elements
	rotatedKeys := append(curKeys[len(curKeys)-1:], curKeys[:len(curKeys)-1]...)

	if err = writeEncryptionConfig(controlConfig, rotatedKeys, true); err != nil {
		return err
	}
	fmt.Println("Encryption keys rotated")

	newEncryptionByte, err := ioutil.ReadFile(controlConfig.Runtime.EncryptionConfig)
	if err != nil {
		return err
	}
	rotateHash := sha256.Sum256(newEncryptionByte)
	// Write new hash to rotate file to prevent rotating twice
	return ioutil.WriteFile(rotateHashFile, rotateHash[:], 0600)
}

func Reencrypt(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	ctx := signals.SetupSignalHandler(context.Background())
	sc, err := server.NewContext(ctx, controlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	updateSecrets(ctx, controlConfig, sc.K8s)

	// Remove last key
	curKeys, err := getEncryptionKeys(controlConfig)
	if err != nil {
		return err
	}

	fmt.Println("Removing key: ", curKeys[len(curKeys)-1])
	curKeys = curKeys[:len(curKeys)-1]
	if err = writeEncryptionConfig(controlConfig, curKeys, true); err != nil {
		return err
	}

	// Cleanup rotate protection file
	rotateHashFile := filepath.Join(controlConfig.DataDir, "cred", "encryption-rotate.sha256")
	os.Remove(rotateHashFile)
	return nil
}

func getServerNodes(ctx context.Context, k8s kubernetes.Interface) ([]corev1.Node, error) {

	nodes, err := k8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var serverNodes []corev1.Node
	for _, node := range nodes.Items {
		if v, ok := node.Labels[server.ControlPlaneRoleLabelKey]; ok && v == "true" {
			serverNodes = append(serverNodes, node)
		}
	}
	return serverNodes, nil
}

func verifyEncryptionHash(nodes []corev1.Node) error {
	var firstHash string
	first := true
	for _, node := range nodes {
		hash, ok := node.Annotations[server.EncryptionConfigHashAnnotation]
		if ok && first {
			firstHash = hash
			first = false
		} else if ok && hash != firstHash {
			return fmt.Errorf("server nodes have different secrets encryption keys")
		}
	}
	return nil
}

func updateSecrets(ctx context.Context, controlConfig config.Control, k8s kubernetes.Interface) error {
	secrets, err := k8s.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, s := range secrets.Items {
		_, err := k8s.CoreV1().Secrets(s.ObjectMeta.Namespace).Update(ctx, &s, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	fmt.Printf("Updated %d secrets\n", len(secrets.Items))
	// Double check no new secrets were added while updating
	newSecrets, err := k8s.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	} else if len(newSecrets.Items) > len(secrets.Items) {
		logrus.Warnf("Addition of %d Secrets while updating existing secrets", len(newSecrets.Items)-len(secrets.Items))
	}
	return nil
}

func getEncryptionProviders(controlConfig config.Control) ([]apiserverconfigv1.ProviderConfiguration, error) {
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

func getEncryptionKeys(controlConfig config.Control) ([]apiserverconfigv1.Key, error) {

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

func writeEncryptionConfig(controlConfig config.Control, keys []apiserverconfigv1.Key, enable bool) error {

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

func getEncryptionHashAnnotations(ctx context.Context, k8s kubernetes.Interface) (string, error) {
	nodeName := os.Getenv("NODE_NAME")
	// Try hostname
	if nodeName == "" {
		return "", fmt.Errorf("NODE_NAME not found")
	}
	node, err := k8s.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return node.Annotations[server.EncryptionConfigHashAnnotation], nil
}

func encryptionStatus(controlConfig config.Control) error {
	providers, err := getEncryptionProviders(controlConfig)
	if os.IsNotExist(err) {
		fmt.Println("Encryption Status: Disabled, no configuration file found")
		return nil
	} else if err != nil {
		return err
	}
	if providers[1].Identity != nil && providers[0].AESCBC != nil {
		fmt.Println("Encryption Status: Enabled")
	} else if providers[0].Identity != nil && providers[1].AESCBC != nil {
		//} else if providers[0].Identity != nil && providers[1].AESCBC != nil || !controlConfig.EncryptSecrets {
		fmt.Println("Encryption Status: Disabled")
	}

	ctx := signals.SetupSignalHandler(context.Background())
	sc, err := server.NewContext(ctx, controlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	cur, err := getEncryptionHashAnnotations(ctx, sc.K8s)
	if err != nil {
		return err
	}
	fmt.Println("Current Encryption Hash: ", cur)

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

func askForConfirmation(message string) bool {
	var s string

	fmt.Printf("%s (y/N): ", message)
	_, err := fmt.Scan(&s)
	if err != nil {
		panic(err)
	}

	s = strings.TrimSpace(s)
	s = strings.ToLower(s)

	if s == "y" || s == "yes" {
		return true
	}
	return false
}
