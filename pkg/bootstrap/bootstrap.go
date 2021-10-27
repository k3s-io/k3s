package bootstrap

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
)

func Handler(bootstrap *config.ControlRuntimeBootstrap) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		Write(rw, bootstrap)
	})
}

func Write(w io.Writer, bootstrap *config.ControlRuntimeBootstrap) error {
	paths, err := objToMap(bootstrap)
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

	return json.NewEncoder(w).Encode(dataMap)
}

func Read(r io.Reader, bootstrap *config.ControlRuntimeBootstrap) error {
	paths, err := objToMap(bootstrap)
	if err != nil {
		return err
	}

	files := map[string][]byte{}
	if err := json.NewDecoder(r).Decode(&files); err != nil {
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

		if err := ioutil.WriteFile(path, data, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", path)
		}
	}

	return nil
}

func objToMap(obj interface{}) (map[string]string, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	data := map[string]string{}
	return data, json.Unmarshal(bytes, &data)
}
