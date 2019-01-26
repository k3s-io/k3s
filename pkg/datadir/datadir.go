package datadir

import (
	"os"

	"github.com/pkg/errors"
	"github.com/rancher/norman/pkg/resolvehome"
)

const (
	DefaultDataDir     = "/var/lib/rancher/k3s"
	DefaultHomeDataDir = "${HOME}/.rancher/k3s"
)

func Resolve(dataDir string) (string, error) {
	if dataDir == "" {
		if os.Getuid() == 0 {
			dataDir = DefaultDataDir
		} else {
			dataDir = DefaultHomeDataDir
		}
	}

	dataDir, err := resolvehome.Resolve(dataDir)
	if err != nil {
		return "", errors.Wrapf(err, "resolving %s", dataDir)
	}

	return dataDir, nil
}
