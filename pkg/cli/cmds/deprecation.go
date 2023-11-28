package cmds

import (
	"fmt"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/mod/semver"
)

// Deprecation tracks flag removal
type Deprecation struct {
	Deprecated string
	Removed    string
}

// Deprecations contains a list of deprecated/removed flags, and the version that they are deprecated/removed in.
var Deprecations = map[string]Deprecation{
	"image-credential-provider-bin-dir": {Deprecated: "v1.28", Removed: "v1.30"},
	"image-credential-provider-config":  {Deprecated: "v1.28", Removed: "v1.30"},
}

// HandleDeprecated handles warning or raising errors for deprecated or removed CLI flags.
func HandleDeprecated(ctx *cli.Context) error {
	for flag, deprecation := range Deprecations {
		if ctx.IsSet(flag) {
			if semver.Compare(version.Version, deprecation.Removed) >= 0 {
				return fmt.Errorf("The %s option was deprecated in %s and removed in %s", flag, deprecation.Deprecated, deprecation.Removed)
			}
			logrus.Warnf("The %s option was deprecated in %s and will be removed in %s", flag, deprecation.Deprecated, deprecation.Removed)
		}
	}
	return nil
}
