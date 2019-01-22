package deploy

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Stage(dataDir string) error {
	os.MkdirAll(dataDir, 0700)

	for _, name := range AssetNames() {
		content, err := Asset(name)
		if err != nil {
			return err
		}

		p := filepath.Join(dataDir, name)
		logrus.Info("Writing manifest: ", p)
		if err := ioutil.WriteFile(p, content, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", name)
		}
	}

	return nil
}
