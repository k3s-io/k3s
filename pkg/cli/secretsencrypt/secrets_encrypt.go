package secretsencrypt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/erikdubbelboer/gspt"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/secretsencrypt"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"k8s.io/utils/pointer"
)

func commandPrep(app *cli.Context, cfg *cmds.Server) (*clientaccess.Info, error) {
	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	gspt.SetProcTitle(os.Args[0] + " secrets-encrypt")

	dataDir, err := server.ResolveDataDir(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	if cfg.Token == "" {
		fp := filepath.Join(dataDir, "token")
		tokenByte, err := ioutil.ReadFile(fp)
		if err != nil {
			return nil, err
		}
		cfg.Token = string(bytes.TrimRight(tokenByte, "\n"))
	}
	return clientaccess.ParseAndValidateTokenForUser(cmds.ServerConfig.ServerURL, cfg.Token, "server")
}

func wrapServerError(err error) error {
	return errors.Wrap(err, "see server log for details")
}

func Enable(app *cli.Context) error {
	var err error
	if err = cmds.InitLogging(); err != nil {
		return err
	}
	info, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	b, err := json.Marshal(server.EncryptionRequest{Enable: pointer.Bool(true)})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/config", b); err != nil {
		return wrapServerError(err)
	}
	fmt.Println("secrets-encryption enabled")
	return nil
}

func Disable(app *cli.Context) error {

	if err := cmds.InitLogging(); err != nil {
		return err
	}
	info, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	b, err := json.Marshal(server.EncryptionRequest{Enable: pointer.Bool(false)})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/config", b); err != nil {
		return wrapServerError(err)
	}
	fmt.Println("secrets-encryption disabled")
	return nil
}

func Status(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	info, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	data, err := info.Get("/v1-" + version.Program + "/encrypt/status")
	if err != nil {
		return wrapServerError(err)
	}
	status := server.EncryptionState{}
	if err := json.Unmarshal(data, &status); err != nil {
		return err
	}

	if strings.ToLower(cmds.ServerConfig.EncryptOutput) == "json" {
		json, err := json.MarshalIndent(status, "", "\t")
		if err != nil {
			return err
		}
		fmt.Println(string(json))
		return nil
	}

	if status.Enable == nil {
		fmt.Println("Encryption Status: Disabled, no configuration file found")
		return nil
	}

	var statusOutput string
	if *status.Enable {
		statusOutput += "Encryption Status: Enabled\n"
	} else {
		statusOutput += "Encryption Status: Disabled\n"
	}
	statusOutput += fmt.Sprintln("Current Rotation Stage:", status.Stage)

	if status.HashMatch {
		statusOutput += fmt.Sprintln("Server Encryption Hashes: All hashes match")
	} else {
		statusOutput += fmt.Sprintf("Server Encryption Hashes: %s\n", status.HashError)
	}

	var tabBuffer bytes.Buffer
	w := tabwriter.NewWriter(&tabBuffer, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "Active\tKey Type\tName\n")
	fmt.Fprintf(w, "------\t--------\t----\n")
	if status.ActiveKey != "" {
		fmt.Fprintf(w, " *\t%s\t%s\n", "AES-CBC", status.ActiveKey)
	}
	for _, k := range status.InactiveKeys {
		fmt.Fprintf(w, "\t%s\t%s\n", "AES-CBC", k)
	}
	w.Flush()
	fmt.Println(statusOutput + tabBuffer.String())
	return nil
}

func Prepare(app *cli.Context) error {
	var err error
	if err = cmds.InitLogging(); err != nil {
		return err
	}
	info, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	b, err := json.Marshal(server.EncryptionRequest{
		Stage: pointer.StringPtr(secretsencrypt.EncryptionPrepare),
		Force: cmds.ServerConfig.EncryptForce,
	})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/config", b); err != nil {
		return wrapServerError(err)
	}
	fmt.Println("prepare completed successfully")
	return nil
}

func Rotate(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	info, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	b, err := json.Marshal(server.EncryptionRequest{
		Stage: pointer.StringPtr(secretsencrypt.EncryptionRotate),
		Force: cmds.ServerConfig.EncryptForce,
	})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/config", b); err != nil {
		return wrapServerError(err)
	}
	fmt.Println("rotate completed successfully")
	return nil
}

func Reencrypt(app *cli.Context) error {
	var err error
	if err = cmds.InitLogging(); err != nil {
		return err
	}
	info, err := commandPrep(app, &cmds.ServerConfig)
	if err != nil {
		return err
	}
	b, err := json.Marshal(server.EncryptionRequest{
		Stage: pointer.StringPtr(secretsencrypt.EncryptionReencryptActive),
		Force: cmds.ServerConfig.EncryptForce,
		Skip:  cmds.ServerConfig.EncryptSkip,
	})
	if err != nil {
		return err
	}
	if err = info.Put("/v1-"+version.Program+"/encrypt/config", b); err != nil {
		return wrapServerError(err)
	}
	fmt.Println("reencryption started")
	return nil
}
