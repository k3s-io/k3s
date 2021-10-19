package secretsencrypt

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/erikdubbelboer/gspt"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/server"
	"github.com/rancher/k3s/pkg/version"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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
	if cmds.ServerConfig.ServerURL == "" {
		cmds.ServerConfig.ServerURL = "https://127.0.0.1:6443"
	}

	if cmds.ServerConfig.Token == "" {
		fp := filepath.Join(controlConfig.DataDir, "token")
		tokenByte, err := ioutil.ReadFile(fp)
		if err != nil {
			return controlConfig, err
		}
		controlConfig.Token = string(bytes.TrimRight(tokenByte, "\n"))
	} else {
		controlConfig.Token = cmds.ServerConfig.Token
	}

	controlConfig.Runtime = &config.ControlRuntime{}
	controlConfig.EncryptSecrets = cfg.EncryptSecrets
	controlConfig.EncryptForce = cfg.EncryptForce

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
	info, err := clientaccess.ParseAndValidateTokenForUser(cmds.ServerConfig.ServerURL, controlConfig.Token, "node")
	if err != nil {
		return err
	}
	data, err := info.Get("/v1-" + version.Program + "/encrypt-status")
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

func Prepare(app *cli.Context) error {
	var err error
	if err = cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	info, err := clientaccess.ParseAndValidateTokenForUser(cmds.ServerConfig.ServerURL, controlConfig.Token, "node")
	if err != nil {
		return err
	}
	if controlConfig.EncryptForce {
		err = info.Put("/v1-" + version.Program + "/encrypt-prepare-force")
	} else {
		err = info.Put("/v1-" + version.Program + "/encrypt-prepare")
	}
	if err != nil {
		return err
	}
	fmt.Println("prepare completed successfully")
	return nil
}

func Rotate(app *cli.Context) error {
	var err error
	if err = cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	info, err := clientaccess.ParseAndValidateTokenForUser(cmds.ServerConfig.ServerURL, controlConfig.Token, "node")
	if err != nil {
		return err
	}
	if controlConfig.EncryptForce {
		err = info.Put("/v1-" + version.Program + "/encrypt-rotate-force")
	} else {
		err = info.Put("/v1-" + version.Program + "/encrypt-rotate")
	}
	if err != nil {
		return err
	}
	fmt.Println("rotate completed successfully")
	return nil
}

func Reencrypt(app *cli.Context) error {
	var err error
	if err = cmds.InitLogging(); err != nil {
		return err
	}
	controlConfig, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	info, err := clientaccess.ParseAndValidateTokenForUser(cmds.ServerConfig.ServerURL, controlConfig.Token, "node")
	if err != nil {
		return err
	}
	if controlConfig.EncryptForce {
		err = info.Put("/v1-" + version.Program + "/encrypt-reencrypt-force")
	} else {
		err = info.Put("/v1-" + version.Program + "/encrypt-reencrypt")
	}
	if err != nil {
		return err
	}
	fmt.Println("reencrypt completed successfully")
	return nil
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
