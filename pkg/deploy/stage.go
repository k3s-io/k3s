// +build !no_stage

package deploy

import (
	"bytes"
	"crypto/sha256"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Stage iterates over the embedded asset list. Assets are checked against the skips map
// to determine if they should not be processed. If they should not be skipped, string substitutions
// are performed using the templateVars map. Finally, content is checksummed to see if the file should
// be rewritten. If the content has changed from the on-disk version, the file is rewritten.
func Stage(dataDir string, templateVars map[string]string, skips map[string]bool) error {
staging:
	for _, name := range AssetNames() {
		nameNoExtension := strings.TrimSuffix(name, filepath.Ext(name))
		if skips[name] || skips[nameNoExtension] {
			continue staging
		}
		namePath := strings.Split(name, string(os.PathSeparator))
		for i := 1; i < len(namePath); i++ {
			subPath := filepath.Join(namePath[0:i]...)
			if skips[subPath] {
				continue staging
			}
		}

		content, err := Asset(name)
		if err != nil {
			return err
		}
		for k, v := range templateVars {
			content = bytes.Replace(content, []byte(k), []byte(v), -1)
		}

		p := filepath.Join(dataDir, name)
		contentHash := sha256.Sum256(content)
		currentHash, err := fileSum256(p)
		if err != nil {
			return err
		}

		if bytes.Equal(contentHash[:], currentHash[:]) {
			logrus.Info("Unchanged manifest: ", p)
			continue
		}

		os.MkdirAll(filepath.Dir(p), 0700)
		logrus.Info("Writing manifest: ", p)
		if err := ioutil.WriteFile(p, content, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", name)
		}
	}

	return nil
}

// fileSum256 returns the sha256 digest of a file at a given path.
func fileSum256(path string) (data []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return data, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return data, err
	}
	return h.Sum(nil), nil
}
