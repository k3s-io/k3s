package cluster

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
)

const (
	backupDest                 = "/tmp/rke1"
	compressedExtension        = "zip"
	stateExtenstion            = "rkestate"
	caCertName                 = "kube-ca"
	requestHeaderCACertName    = "kube-apiserver-requestheader-ca"
	etcdClientCACertName       = "kube-etcd-client-ca"
	serviceAccountTokenKeyName = "kube-service-account-token"
	certPEM                    = "certificatePEM"
	keyPEM                     = "keyPEM"

	certType = iota
	keyType
)

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0700)

	for _, f := range r.File {
		err := extract(f, dest)
		if err != nil {
			return err
		}
	}

	return nil
}

func extract(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		if err := rc.Close(); err != nil {
			panic(err)
		}
	}()

	path := filepath.Join(dest, f.Name)

	if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path: %s", path)
	}

	if f.FileInfo().IsDir() {
		os.MkdirAll(path, f.Mode())
	} else {
		os.MkdirAll(filepath.Dir(path), f.Mode())
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		defer func() {
			if err := f.Close(); err != nil {
				panic(err)
			}
		}()

		_, err = io.Copy(f, rc)
		if err != nil {
			return err
		}
	}
	return nil
}

func isCompressed(filename string) bool {
	return strings.HasSuffix(filename, fmt.Sprintf(".%s", compressedExtension))
}

func isStateFile(filename string) bool {
	return strings.HasSuffix(filename, fmt.Sprintf(".%s", stateExtenstion))
}

func recoverCerts(ctx context.Context, runtime *config.ControlRuntime, stateFile string) error {
	var state map[string]interface{}
	stateJSON, err := ioutil.ReadFile(stateFile)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(stateJSON, &state); err != nil {
		return err
	}
	currentState := state["currentState"].(map[string]interface{})
	certs := currentState["certificatesBundle"].(map[string]interface{})

	return writeCertBundle(runtime, certs)
}

func writeCertBundle(runtime *config.ControlRuntime, certBundle map[string]interface{}) error {
	for certName, cert := range certBundle {
		currentCert := cert.(map[string]interface{})
		switch certName {
		case caCertName:
			if err := writeFile(
				currentCert, certType, runtime.ControlRuntimeBootstrap.ClientCA,
				runtime.ControlRuntimeBootstrap.ETCDPeerCA,
				runtime.ControlRuntimeBootstrap.ServerCA); err != nil {
				return err
			}
			if err := writeFile(
				currentCert, keyType, runtime.ControlRuntimeBootstrap.ClientCAKey,
				runtime.ControlRuntimeBootstrap.ETCDPeerCAKey,
				runtime.ControlRuntimeBootstrap.ServerCAKey); err != nil {
				return err
			}
		case requestHeaderCACertName:
			if err := writeFile(
				currentCert, certType, runtime.ControlRuntimeBootstrap.RequestHeaderCA); err != nil {
				return err
			}
			if err := writeFile(
				currentCert, keyType, runtime.ControlRuntimeBootstrap.RequestHeaderCAKey); err != nil {
				return err
			}
		case etcdClientCACertName:
			if err := writeFile(
				currentCert, certType, runtime.ControlRuntimeBootstrap.ETCDServerCA); err != nil {
				return err
			}
			if err := writeFile(
				currentCert, keyType, runtime.ControlRuntimeBootstrap.RequestHeaderCAKey); err != nil {
				return err
			}
		case serviceAccountTokenKeyName:
			if err := writeFile(
				currentCert, keyType, runtime.ControlRuntimeBootstrap.ServiceKey); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeFile(cert map[string]interface{}, fileType int, certPaths ...string) error {
	for _, path := range certPaths {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return errors.Wrapf(err, "failed to mkdir %s", filepath.Dir(path))
		}
		if fileType == certType {
			if err := ioutil.WriteFile(path, []byte(cert[certPEM].(string)), 0600); err != nil {
				return errors.Wrapf(err, "failed to write to %s", path)
			}
		} else if fileType == keyType {
			if err := ioutil.WriteFile(path, []byte(cert[keyPEM].(string)), 0600); err != nil {
				return errors.Wrapf(err, "failed to write to %s", path)
			}
		}
	}
	return nil
}

func findStateFile(destDir string) (string, error) {
	fileList, err := fileList(destDir)
	if err != nil {
		return "", err
	}
	for _, file := range fileList {
		if isStateFile(file) {
			return file, nil
		}
	}
	return "", fmt.Errorf("file not found")
}

func findSnapshotFile(destDir string) (string, error) {
	fileList, err := fileList(destDir)
	if err != nil {
		return "", err
	}
	for _, file := range fileList {
		f, err := os.Stat(file)
		if err != nil {
			return "", nil
		}
		if f.IsDir() {
			continue
		}
		if !isStateFile(file) {
			return file, nil
		}
	}
	return "", fmt.Errorf("snapshot file not found")
}

func fileList(destDir string) ([]string, error) {
	fileList := make([]string, 0)
	e := filepath.Walk(destDir, func(path string, f os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return err
	})

	if e != nil {
		return nil, fmt.Errorf("failed to search content of snapshot")
	}
	return fileList, nil
}
