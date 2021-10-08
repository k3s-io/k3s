package encrypt

import (
	"encoding/json"
	"fmt"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/urfave/cli"
)

func pp(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func Run(ctx *cli.Context) error {
	return nil
}

func Prepare(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return prepare(app, &cmds.ServerConfig)
}

func prepare(app *cli.Context, cfg *cmds.Server) error {
	fmt.Print(pp(cfg))
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
