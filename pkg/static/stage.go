// +build !no_stage

package static

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Stage(dataDir string, isStageTraefik bool) error {
	for _, name := range AssetNames() {
		if !isStageTraefik && strings.HasPrefix(name, "charts/traefik-") {
			logrus.WithField("name", name).Info("Skip staging")
			continue
		}
		content, err := Asset(name)
		if err != nil {
			return err
		}
		p := filepath.Join(dataDir, name)
		logrus.Info("Writing static file: ", p)
		os.MkdirAll(filepath.Dir(p), 0700)
		if err := ioutil.WriteFile(p, content, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", name)
		}
	}

	return nil
}
