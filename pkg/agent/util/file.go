package util

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func WriteFile(name string, content string) error {
	os.MkdirAll(filepath.Dir(name), 0755)
	err := ioutil.WriteFile(name, []byte(content), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing %s", name)
	}
	return nil
}
