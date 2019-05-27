package deploy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	errors2 "github.com/pkg/errors"
	v12 "github.com/rancher/k3s/pkg/apis/k3s.cattle.io/v1"
	v1 "github.com/rancher/k3s/pkg/generated/controllers/k3s.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/rancher/wrangler/pkg/objectset"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	ns       = "kube-system"
	startKey = "_start_"
)

func WatchFiles(ctx context.Context, apply apply.Apply, addons v1.AddonController, bases ...string) error {
	w := &watcher{
		apply:      apply,
		addonCache: addons.Cache(),
		addons:     addons,
		bases:      bases,
	}

	addons.Enqueue("", startKey)
	addons.OnChange(ctx, "addon-start", func(key string, _ *v12.Addon) (*v12.Addon, error) {
		if key == startKey {
			go w.start(ctx)
		}
		return nil, nil
	})

	return nil
}

type watcher struct {
	apply      apply.Apply
	addonCache v1.AddonCache
	addons     v1.AddonClient
	bases      []string
}

func (w *watcher) start(ctx context.Context) {
	force := true
	for {
		if err := w.listFiles(force); err == nil {
			force = false
		} else {
			logrus.Errorf("failed to process config: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}

func (w *watcher) listFiles(force bool) error {
	var errs []error
	for _, base := range w.bases {
		if err := w.listFilesIn(base, force); err != nil {
			errs = append(errs, err)
		}

	}
	return merr.NewErrors(errs...)
}

func (w *watcher) listFilesIn(base string, force bool) error {
	files, err := ioutil.ReadDir(base)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	skips := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".skip") {
			skips[strings.TrimSuffix(file.Name(), ".skip")] = true
		}
	}

	var errs []error
	for _, file := range files {
		if skipFile(file.Name(), skips) {
			continue
		}
		p := filepath.Join(base, file.Name())
		if err := w.deploy(p, !force); err != nil {
			errs = append(errs, errors2.Wrapf(err, "failed to process %s", p))
		}
	}

	return merr.NewErrors(errs...)
}

func (w *watcher) deploy(path string, compareChecksum bool) error {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	name := name(path)
	addon, err := w.addon(name)
	if err != nil {
		return err
	}

	checksum := checksum(content)
	if compareChecksum && checksum == addon.Spec.Checksum {
		logrus.Debugf("Skipping existing deployment of %s, check=%v, checksum %s=%s", path, compareChecksum, checksum, addon.Spec.Checksum)
		return nil
	}

	objectSet, err := objectSet(content)
	if err != nil {
		return err
	}

	if err := w.apply.WithOwner(&addon).Apply(objectSet); err != nil {
		return err
	}

	addon.Spec.Source = path
	addon.Spec.Checksum = checksum
	addon.Status.GVKs = nil

	if addon.UID == "" {
		_, err := w.addons.Create(&addon)
		return err
	}

	_, err = w.addons.Update(&addon)
	return err
}

func (w *watcher) addon(name string) (v12.Addon, error) {
	addon, err := w.addonCache.Get(ns, name)
	if errors.IsNotFound(err) {
		addon = v12.NewAddon(ns, name, v12.Addon{})
	} else if err != nil {
		return v12.Addon{}, err
	}
	return *addon, nil
}

func objectSet(content []byte) (*objectset.ObjectSet, error) {
	objs, err := yamlToObjects(bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	os := objectset.NewObjectSet()
	os.Add(objs...)
	return os, nil
}

func name(path string) string {
	name := filepath.Base(path)
	return strings.SplitN(name, ".", 2)[0]
}

func checksum(bytes []byte) string {
	d := sha256.Sum256(bytes)
	return hex.EncodeToString(d[:])
}

func isEmptyYaml(yaml []byte) bool {
	isEmpty := true
	lines := bytes.Split(yaml, []byte("\n"))
	for _, l := range lines {
		s := bytes.TrimSpace(l)
		if string(s) != "---" && !bytes.HasPrefix(s, []byte("#")) && string(s) != "" {
			isEmpty = false
		}
	}
	return isEmpty
}

func yamlToObjects(in io.Reader) ([]runtime.Object, error) {
	var result []runtime.Object
	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(in, 4096))
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if !isEmptyYaml(raw) {
			obj, err := toObjects(raw)
			if err != nil {
				return nil, err
			}

			result = append(result, obj...)
		}
	}

	return result, nil
}

func toObjects(bytes []byte) ([]runtime.Object, error) {
	bytes, err := yamlDecoder.ToJSON(bytes)
	if err != nil {
		return nil, err
	}

	obj, _, err := unstructured.UnstructuredJSONScheme.Decode(bytes, nil, nil)
	if err != nil {
		return nil, err
	}

	if l, ok := obj.(*unstructured.UnstructuredList); ok {
		var result []runtime.Object
		for _, obj := range l.Items {
			copy := obj
			result = append(result, &copy)
		}
		return result, nil
	}

	return []runtime.Object{obj}, nil
}

func skipFile(fileName string, skips map[string]bool) bool {
	switch {
	case strings.HasPrefix(fileName, "."):
		return true
	case skips[fileName]:
		return true
	case strings.HasSuffix(fileName, ".json"):
		return false
	case strings.HasSuffix(fileName, ".yml"):
		return false
	case strings.HasSuffix(fileName, ".yaml"):
		return false
	default:
		return true
	}
}
