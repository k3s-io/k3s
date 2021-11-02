package secretsencrypt

import (
	"bytes"
	"encoding/json"
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
	controlConfig.EncryptForce = cfg.EncryptForce

	return controlConfig, nil
}

func Run(app *cli.Context) error {
	fmt.Println("This command does nothing, use the subcommands")
	return nil
}

func Enable(app *cli.Context) error {
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
	b, err := json.Marshal(server.EncryptionRequest{Toggle: true})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/toggle", b); err != nil {
		return err
	}
	fmt.Println("secrets-encryption enabled")
	return nil
}

func Disable(app *cli.Context) error {
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
	b, err := json.Marshal(server.EncryptionRequest{Toggle: false})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/toggle", b); err != nil {
		return err
	}
	fmt.Println("secrets-encryption disabled")
	return nil
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
	data, err := info.Get("/v1-" + version.Program + "/encrypt/status")
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
	b, err := json.Marshal(server.EncryptionRequest{
		Stage: server.EncryptionPrepare,
		Force: controlConfig.EncryptForce,
	})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/stage", b); err != nil {
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
	b, err := json.Marshal(server.EncryptionRequest{
		Stage: server.EncryptionRotate,
		Force: controlConfig.EncryptForce,
	})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/stage", b); err != nil {
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
	b, err := json.Marshal(server.EncryptionRequest{
		Stage: server.EncryptionReencrypt,
		Force: controlConfig.EncryptForce,
	})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/stage", b); err != nil {
		return err
	}
	fmt.Println("reencrypt completed successfully")
	return nil
}
