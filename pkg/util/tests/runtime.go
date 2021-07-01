package tests

import (
	"os"
	"path/filepath"

	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/control/deps"
)

// GenerateDataDir creates a temporary directory at "/tmp/k3s/<RANDOM_STRING>/".
// The latest directory created with this function is soft linked to "/tmp/k3s/latest/".
// This allows tests to replicate the "/var/lib/rancher/k3s" directory structure.
func GenerateDataDir(cnf *config.Control) error {
	if err := os.MkdirAll(cnf.DataDir, 0700); err != nil {
		return err
	}
	testDir, err := os.MkdirTemp(cnf.DataDir, "*")
	if err != nil {
		return err
	}
	// Remove old symlink and add new one
	os.Remove(filepath.Join(cnf.DataDir, "latest"))
	if err = os.Symlink(testDir, filepath.Join(cnf.DataDir, "latest")); err != nil {
		return err
	}
	cnf.DataDir = testDir
	cnf.DataDir, err = filepath.Abs(cnf.DataDir)
	if err != nil {
		return err
	}
	return nil
}

// CleanupDataDir removes the associated "/tmp/k3s/<RANDOM_STRING>"
// directory.
func CleanupDataDir(cnf *config.Control) {
	os.RemoveAll(cnf.DataDir)
}

// GenerateRuntime creates a temporary data dir and configures
// config.ControlRuntime with all the appropriate certificate keys.
func GenerateRuntime(cnf *config.Control) error {
	runtime := &config.ControlRuntime{}
	if err := GenerateDataDir(cnf); err != nil {
		return err
	}

	os.MkdirAll(filepath.Join(cnf.DataDir, "tls"), 0700)
	os.MkdirAll(filepath.Join(cnf.DataDir, "cred"), 0700)

	deps.FillRuntimeCerts(cnf, runtime)

	if err := deps.GenServerDeps(cnf, runtime); err != nil {
		return err
	}
	cnf.Runtime = runtime
	return nil
}
