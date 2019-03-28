package deploy

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Stage(dataDir string, templateVars map[string]string, skipList []string) error {
	os.MkdirAll(dataDir, 0700)

	skips := map[string]bool{}
	for _, skip := range skipList {
		skips[skip] = true
	}

	for _, name := range AssetNames() {
		if skips[name] {
			continue
		}
		content, err := Asset(name)
		if err != nil {
			return err
		}
		for k, v := range templateVars {
			content = bytes.Replace(content, []byte(k), []byte(v), -1)
		}
		p := filepath.Join(dataDir, name)
		logrus.Info("Writing manifest: ", p)
		if err := ioutil.WriteFile(p, content, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", name)
		}
	}

	return nil
}
