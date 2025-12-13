//go:build !no_stage

package static

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/util/bindata"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

//go:embed embed/*
var embedFS embed.FS

var bd = bindata.Bindata{FS: &embedFS, Prefix: "embed"}

func Stage(dataDir string) error {
	for _, name := range bd.AssetNames() {
		content, err := bd.Asset(name)
		if err != nil {
			return err
		}
		p := filepath.Join(dataDir, name)
		logrus.Info("Writing static file: ", p)
		os.MkdirAll(filepath.Dir(p), 0700)
		if err := os.WriteFile(p, content, 0600); err != nil {
			return pkgerrors.WithMessagef(err, "failed to write to %s", name)
		}
	}

	return nil
}
