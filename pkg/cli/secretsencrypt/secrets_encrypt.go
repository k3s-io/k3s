package secretsencrypt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

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
	controlConfig.Runtime.EncryptionState = filepath.Join(controlConfig.DataDir, "cred", "encryption-state.json")
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

	providers, err := server.GetEncryptionProviders(controlConfig)
	if err != nil {
		return err
	}
	if len(providers) > 2 {
		return fmt.Errorf("more than 2 providers (%d) found in secrets encryption", len(providers))
	}
	curKeys, err := server.GetEncryptionKeys(controlConfig)
	if err != nil {
		return err
	}
	if providers[1].Identity != nil && providers[0].AESCBC != nil {
		fmt.Println("Disabling secrets encryption")
		return server.WriteEncryptionConfig(controlConfig, curKeys, false)
	} else if providers[0].Identity != nil && providers[1].AESCBC != nil {
		fmt.Println("Enabling secrets encryption")
		return server.WriteEncryptionConfig(controlConfig, curKeys, true)
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
	restConfig, err := clientcmd.BuildConfigFromFlags("", controlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	k8s := kubernetes.NewForConfigOrDie(restConfig)
	serverNodes, err := getServerNodes(ctx, k8s)
	if err != nil {
		return err
	}
	if err := verifyEncryptionHash(serverNodes); err != nil {
		return err
	}

	stage, key, err := server.GetEncryptionState(controlConfig)
	if err != nil {
		return err
	} else if stage != server.Start && stage != server.Reencrypt {
		return fmt.Errorf("error, incorrect stage %s found with key %s", stage, key.Name)
	}

	curKeys, err := server.GetEncryptionKeys(controlConfig)
	if err != nil {
		return err
	}

	server.AppendNewEncryptionKey(&curKeys)
	fmt.Println("Adding key: ", curKeys[len(curKeys)-1])

	if err := server.WriteEncryptionConfig(controlConfig, curKeys, true); err != nil {
		return err
	}
	return server.WriteEncryptionState(controlConfig, server.Prepare, curKeys[0])
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}

	stage, key, err := server.GetEncryptionState(controlConfig)
	if err != nil {
		return err
	} else if stage != server.Prepare {
		return fmt.Errorf("error, incorrect stage %s found with key %s", stage, key.Name)
	}

	curKeys, err := server.GetEncryptionKeys(controlConfig)
	if err != nil {
		return err
	}

	// Right rotate elements
	rotatedKeys := append(curKeys[len(curKeys)-1:], curKeys[:len(curKeys)-1]...)

	if err = server.WriteEncryptionConfig(controlConfig, rotatedKeys, true); err != nil {
		return err
	}
	fmt.Println("Encryption keys rotated")
	return server.WriteEncryptionState(controlConfig, server.Rotate, curKeys[0])
}

func Reencrypt(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}

	stage, key, err := server.GetEncryptionState(controlConfig)
	if err != nil {
		return err
	} else if stage != server.Rotate {
		return fmt.Errorf("error, incorrect stage %s found with key %s", stage, key.Name)
	}

	ctx := signals.SetupSignalHandler(context.Background())
	sc, err := server.NewContext(ctx, controlConfig.Runtime.KubeConfigAdmin)
	if err != nil {
		return err
	}
	updateSecrets(ctx, controlConfig, sc.K8s)

	// Remove last key
	curKeys, err := server.GetEncryptionKeys(controlConfig)
	if err != nil {
		return err
	}

	fmt.Println("Removing key: ", curKeys[len(curKeys)-1])
	curKeys = curKeys[:len(curKeys)-1]
	if err = server.WriteEncryptionConfig(controlConfig, curKeys, true); err != nil {
		return err
	}

	// Cleanup rotate protection file
	return server.WriteEncryptionState(controlConfig, server.Reencrypt, curKeys[0])
}

func getServerNodes(ctx context.Context, k8s *kubernetes.Clientset) ([]corev1.Node, error) {

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
	return nil
}

func encryptionStatus(controlConfig config.Control) error {
	providers, err := server.GetEncryptionProviders(controlConfig)
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
	restConfig, err := clientcmd.BuildConfigFromFlags("", controlConfig.Runtime.KubeConfigAdmin)
	k8s := kubernetes.NewForConfigOrDie(restConfig)

	if err != nil {
		return err
	}
	cur, err := server.GetEncryptionHashAnnotations(ctx, k8s)
	if err != nil {
		return err
	}
	fmt.Println("Current Encryption Hash: ", cur)

	stage, _, err := server.GetEncryptionState(controlConfig)
	if err != nil {
		return err
	}
	fmt.Println("Current Rotation Stage:", stage)

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
