package bootstrap

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/config"
)

// Handler
func Handler(bootstrap *config.ControlRuntimeBootstrap) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		ReadFromDisk(rw, bootstrap)
	})
}

// ReadFromDisk
func ReadFromDisk(w io.Writer, bootstrap *config.ControlRuntimeBootstrap) error {
	paths, err := ObjToMap(bootstrap)
	if err != nil {
		return nil
	}

	dataMap := make(map[string]File)
	for pathKey, path := range paths {
		if path == "" {
			continue
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "failed to read %s", path)
		}

		info, err := os.Stat(path)
		if err != nil {
			return err
		}

		dataMap[pathKey] = File{
			Timestamp: info.ModTime(),
			Content:   data,
		}
	}

	return json.NewEncoder(w).Encode(dataMap)
}

// File is a representation of a certificate
// or key file within the bootstrap context that contains
// the contents of the file as well as a timestamp from
// when the file was last modified.
type File struct {
	Timestamp time.Time
	Content   []byte
}

type DataFormat map[string]File

// WriteToDiskFromStorage writes the contents of the given reader to the paths
// derived from within the ControlRuntimeBootstrap.
func WriteToDiskFromStorage(r io.Reader, bootstrap *config.ControlRuntimeBootstrap) error {
	paths, err := ObjToMap(bootstrap)
	if err != nil {
		return err
	}

	files := make(map[string]File)
	if err := json.NewDecoder(r).Decode(&files); err != nil {
		return err
	}

	for pathKey, bsf := range files {
		path, ok := paths[pathKey]
		if !ok {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return errors.Wrapf(err, "failed to mkdir %s", filepath.Dir(path))
		}
		if err := ioutil.WriteFile(path, bsf.Content, 0600); err != nil {
			return errors.Wrapf(err, "failed to write to %s", path)
		}
	}

	return nil
}

func ObjToMap(obj interface{}) (map[string]string, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	data := map[string]string{}
	return data, json.Unmarshal(bytes, &data)
}
