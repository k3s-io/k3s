package util

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func WriteFile(name string, content string) error {
	os.MkdirAll(filepath.Dir(name), 0755)
	err := os.WriteFile(name, []byte(content), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing %s", name)
	}
	return nil
}

// CopyFile copies the contents of a file.
// If ignoreNotExist is true, no error is returned if the source file does not exist.
func CopyFile(sourceFile string, destinationFile string, ignoreNotExist bool) error {
	os.MkdirAll(filepath.Dir(destinationFile), 0755)
	input, err := os.ReadFile(sourceFile)
	if errors.Is(err, os.ErrNotExist) && ignoreNotExist {
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "copying %s to %s", sourceFile, destinationFile)
	}
	err = os.WriteFile(destinationFile, input, 0644)
	if err != nil {
		return errors.Wrapf(err, "copying %s to %s", sourceFile, destinationFile)
	}
	return nil
}

// kubeadm utility cribbed from:
// https://github.com/kubernetes/kubernetes/blob/v1.25.4/cmd/kubeadm/app/util/copy.go
// Copying this instead of importing from kubeadm saves about 4mb of binary size.

// CopyDir copies the content of a folder
func CopyDir(src string, dst string) error {
	stderr := &bytes.Buffer{}
	cmd := exec.Command("cp", "-r", src, dst)
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return errors.New(strings.TrimSpace(stderr.String()))
	}
	return nil
}
