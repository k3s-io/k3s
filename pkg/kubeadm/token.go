package kubeadm

import (
	"errors"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
)

var (
	NodeBootstrapTokenAuthGroup = "system:bootstrappers:" + version.Program + ":default-node-token"
)

// SetDefaults ensures that the default values are set on the token configuration.
// These are set here, rather than in the default Token struct, to avoid
// importing the cluster-bootstrap packages into the CLI.
func SetDefaults(clx *cli.Context, cfg *cmds.Token) error {
	if !clx.IsSet("groups") {
		cfg.Groups = []string{NodeBootstrapTokenAuthGroup}
	}

	if !clx.IsSet("usages") {
		cfg.Usages = bootstrapapi.KnownTokenUsages
	}

	if cfg.Output == "" {
		cfg.Output = "text"
	} else {
		switch cfg.Output {
		case "text", "json", "yaml":
		default:
			return errors.New("invalid output format: " + cfg.Output)
		}
	}

	args := clx.Args()
	if len(args) > 0 {
		cfg.Token = args[0]
	}

	if cfg.Token == "" {
		var err error
		cfg.Token, err = bootstraputil.GenerateBootstrapToken()
		if err != nil {
			return err
		}
	}

	return nil
}
