package datadir

import (
	"os"

	"github.com/pkg/errors"
	"github.com/rancher/norman/pkg/resolvehome"
)

func Resolve(dataDir string) (string, error) {
	if dataDir == "" {
		if os.Getuid() == 0 {
			dataDir = "/var/lib/rancher/k3s"
		} else {
			dataDir = "${HOME}/.rancher/k3s"
		}
	}

	dataDir, err := resolvehome.Resolve(dataDir)
	if err != nil {
		return "", errors.Wrapf(err, "resolving %s", dataDir)
	}

	return dataDir, nil
}
