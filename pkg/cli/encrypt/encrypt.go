package encrypt

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/wrangler/pkg/signals"
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
	configControl, err := commandPrep(&cmds.ServerConfig)
	if err != nil {
		return err
	}
	ctx := signals.SetupSignalHandler(context.Background())
	sc, err := server.NewContext(ctx, configControl.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	return encryptionStatus(ctx, configControl, sc.K8s)
}

func Prepare(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	sc := &cmds.ServerConfig
	fmt.Print(pp(sc))

	return nil
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return nil
}

func Reencrypt(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return nil
}

func encryptionStatus(ctx context.Context, configControl config.Control, k8s kubernetes.Interface) error {

	if !configControl.EncryptSecrets {
		fmt.Println("Encryption Status: Disabled")
		// return nil
	}
	fmt.Println("Encryption Status: Enabled")
	curEncryptionByte, err := ioutil.ReadFile(configControl.Runtime.EncryptionConfig)
	// curEncryptionByte, err := ioutil.ReadFile("/home/dereknola/tmp/ec.json")
	if err != nil {
		return err
	}

	curEncryption := apiserverconfigv1.EncryptionConfiguration{}
	err = json.Unmarshal(curEncryptionByte, &curEncryption)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	providers := curEncryption.Resources[0].Providers
	fmt.Fprintf(w, "Key Type\tName\tSecret\n")

	for _, k := range providers {
		if k.AESCBC != nil {
			for _, aesKey := range k.AESCBC.Keys {
				fmt.Fprintf(w, "%s\t%s\t%s\n", "AES-CBC", aesKey.Name, aesKey.Secret)
			}
		}
		if k.Identity != nil {
			fmt.Fprintf(w, "Identity\tidentity\tN/A\n")
		}
	}

	return w.Flush()
}

func readSecrets(ctx context.Context, configControl config.Control, k8s kubernetes.Interface) error {
	secrets, err := k8s.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	fmt.Println(pp(secrets))
	return nil
}

// func appendEncryptionKey(runtime *config.ControlRuntime) error {
// 	if s, err := os.Stat(runtime.EncryptionConfig); err == nil && s.Size() > 0 {
// 		return nil
// 	}

// 	aescbcKey := make([]byte, aescbcKeySize)
// 	_, err := rand.Read(aescbcKey)
// 	if err != nil {
// 		return err
// 	}
// 	encodedKey := base64.StdEncoding.EncodeToString(aescbcKey)

// 	encConfig := apiserverconfigv1.EncryptionConfiguration{
// 		TypeMeta: metav1.TypeMeta{
// 			Kind:       "EncryptionConfiguration",
// 			APIVersion: "apiserver.config.k8s.io/v1",
// 		},
// 		Resources: []apiserverconfigv1.ResourceConfiguration{
// 			{
// 				Resources: []string{"secrets"},
// 				Providers: []apiserverconfigv1.ProviderConfiguration{
// 					{
// 						AESCBC: &apiserverconfigv1.AESConfiguration{
// 							Keys: []apiserverconfigv1.Key{
// 								{
// 									Name:   "aescbckey",
// 									Secret: encodedKey,
// 								},
// 							},
// 						},
// 					},
// 					{
// 						Identity: &apiserverconfigv1.IdentityConfiguration{},
// 					},
// 				},
// 			},
// 		},
// 	}
// 	jsonfile, err := json.Marshal(encConfig)
// 	if err != nil {
// 		return err
// 	}
// 	return ioutil.WriteFile(runtime.EncryptionConfig, jsonfile, 0600)
// }
