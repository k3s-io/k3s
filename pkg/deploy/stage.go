//go:build !no_stage
// +build !no_stage

package deploy

import (
	"bytes"
	"embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/k3s-io/k3s/pkg/util/bindata"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

//go:embed embed/*
var embedFS embed.FS

var bd = bindata.Bindata{FS: &embedFS, Prefix: "embed"}

func Stage(dataDir string, templateVars map[string]string, skips map[string]bool) error {
staging:
	for _, name := range bd.AssetNames() {
		nameNoExtension := strings.TrimSuffix(name, filepath.Ext(name))
		if skips[name] || skips[nameNoExtension] {
			continue staging
		}
		// nb: embed always uses forward slash as a path separator
		namePath := strings.Split(name, "/")
		for i := 1; i < len(namePath); i++ {
			subPath := filepath.Join(namePath[0:i]...)
			if skips[subPath] {
				continue staging
			}
		}

		content, err := bd.Asset(name)
		if err != nil {
			return err
		}
		for k, v := range templateVars {
			content = bytes.Replace(content, []byte(k), []byte(v), -1)
		}
		p := filepath.Join(dataDir, name)
		os.MkdirAll(filepath.Dir(p), 0700)
		logrus.Info("Writing manifest: ", p)
		if err := os.WriteFile(p, content, 0600); err != nil {
			return pkgerrors.WithMessagef(err, "failed to write to %s", name)
		}
	}

	return nil
}
