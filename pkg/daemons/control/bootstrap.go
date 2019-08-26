package control

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/kine/pkg/client"
	"github.com/sirupsen/logrus"
)

const (
	k3sRuntimeEtcdPath = "/k3s/runtime"
)

// fetchBootstrapData copies the bootstrap data (certs, keys, passwords)
// from etcd to individual files specified by cfg.Runtime.
func fetchBootstrapData(ctx context.Context, cfg *config.Control, c client.Client) error {
	logrus.Info("Fetching bootstrap data from etcd")
	gr, err := c.Get(ctx, k3sRuntimeEtcdPath)
	if err != nil {
		return err
	}
	if gr.Modified == 0 {
		return nil
	}

	paths, err := objToMap(&cfg.Runtime.ControlRuntimeBootstrap)
	if err != nil {
		return err
	}

	files := map[string][]byte{}
	if err := json.Unmarshal(gr.Data, &files); err != nil {
		return err
	}

	for pathKey, data := range files {
		path, ok := paths[pathKey]
		if !ok {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return errors.Wrapf(err, "failed to mkdir %s", filepath.Dir(path))
		}

		if err := ioutil.WriteFile(path, data, 0700); err != nil {
			return errors.Wrapf(err, "failed to write to %s", path)
		}
	}

	return nil
}

// storeBootstrapData copies the bootstrap data in the opposite direction to
// fetchBootstrapData.
func storeBootstrapData(ctx context.Context, cfg *config.Control, client client.Client) error {
	if cfg.BootstrapReadOnly {
		return nil
	}

	paths, err := objToMap(&cfg.Runtime.ControlRuntimeBootstrap)
	if err != nil {
		return nil
	}

	dataMap := map[string][]byte{}
	for pathKey, path := range paths {
		if path == "" {
			continue
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "failed to read %s", path)
		}

		dataMap[pathKey] = data
	}

	bytes, err := json.Marshal(dataMap)
	if err != nil {
		return err
	}

	return client.Put(ctx, k3sRuntimeEtcdPath, bytes)
}

func objToMap(obj interface{}) (map[string]string, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	data := map[string]string{}
	return data, json.Unmarshal(bytes, &data)
}
