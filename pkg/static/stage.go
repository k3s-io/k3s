package static

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Stage(dataDir string) error {
	for _, osSpecificName := range AssetNames() {
		if skipOsFileName(osSpecificName) {
			continue
		}
		content, err := Asset(osSpecificName)
		if err != nil {
			return err
		}
		p := filepath.Join(dataDir, convertOsFileName(osSpecificName))
		logrus.Info("Writing static file: ", p)
		os.MkdirAll(filepath.Dir(p), 0700)
		if err := ioutil.WriteFile(p, content, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", osSpecificName)
		}
	}

	return nil
}
