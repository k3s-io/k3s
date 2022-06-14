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

func CopyFile(sourceFile string, destinationFile string) error {
	os.MkdirAll(filepath.Dir(destinationFile), 0755)
	input, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		return errors.Wrapf(err, "copying %s to %s", sourceFile, destinationFile)
	}
	err = ioutil.WriteFile(destinationFile, input, 0644)
	if err != nil {
		return errors.Wrapf(err, "copying %s to %s", sourceFile, destinationFile)
	}
	return nil
}
