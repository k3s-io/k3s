package util

import (
	"errors"
	"os"
	"path/filepath"

	pkgerrors "github.com/pkg/errors"
)

func WriteFile(name string, content string) error {
	os.MkdirAll(filepath.Dir(name), 0755)
	err := os.WriteFile(name, []byte(content), 0644)
	if err != nil {
		return pkgerrors.WithMessagef(err, "writing %s", name)
	}
	return nil
}

func CopyFile(sourceFile string, destinationFile string, ignoreNotExist bool) error {
	os.MkdirAll(filepath.Dir(destinationFile), 0755)
	input, err := os.ReadFile(sourceFile)
	if errors.Is(err, os.ErrNotExist) && ignoreNotExist {
		return nil
	} else if err != nil {
		return pkgerrors.WithMessagef(err, "copying %s to %s", sourceFile, destinationFile)
	}
	err = os.WriteFile(destinationFile, input, 0644)
	if err != nil {
		return pkgerrors.WithMessagef(err, "copying %s to %s", sourceFile, destinationFile)
	}
	return nil
}
