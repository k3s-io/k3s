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

	v1 "github.com/rancher/k3s/types/apis/k3s.cattle.io/v1"
	"github.com/rancher/norman"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/pkg/objectset"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

const (
	ns       = "kube-system"
	startKey = "_start_"
)

func WatchFiles(ctx context.Context, bases ...string) error {
	server := norman.GetServer(ctx)
	addons := v1.ClientsFrom(ctx).Addon

	w := &watcher{
		addonCache: addons.Cache(),
		addons:     addons,
		bases:      bases,
		restConfig: *server.Runtime.LocalConfig,
		discovery:  server.K8sClient.Discovery(),
		clients:    map[schema.GroupVersionKind]*objectclient.ObjectClient{},
	}

	addons.Enqueue("", startKey)
	addons.Interface().AddHandler(ctx, "addon-start", func(key string, _ *v1.Addon) (runtime.Object, error) {
		if key == startKey {
			go w.start(ctx)
		}
		return nil, nil
	})

	return nil
}

type watcher struct {
	addonCache v1.AddonClientCache
	addons     v1.AddonClient
	bases      []string
	restConfig rest.Config
	discovery  discovery.DiscoveryInterface
	clients    map[schema.GroupVersionKind]*objectclient.ObjectClient
	namespaced map[schema.GroupVersionKind]bool
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
	return types.NewErrors(errs...)
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
		if strings.HasSuffix(file.Name(), ".skip") || skips[file.Name()] {
			continue
		}
		p := filepath.Join(base, file.Name())
		if err := w.deploy(p, !force); err != nil {
			errs = append(errs, errors2.Wrapf(err, "failed to process %s", p))
		}
	}

	return types.NewErrors(errs...)
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

	clients, err := w.apply(addon, objectSet)
	if err != nil {
		return err
	}

	if w.clients == nil {
		w.clients = map[schema.GroupVersionKind]*objectclient.ObjectClient{}
	}

	addon.Spec.Source = path
	addon.Spec.Checksum = checksum
	addon.Status.GVKs = nil

	for gvk, client := range clients {
		addon.Status.GVKs = append(addon.Status.GVKs, gvk)
		w.clients[gvk] = client
	}

	if addon.UID == "" {
		_, err := w.addons.Create(&addon)
		return err
	}

	_, err = w.addons.Update(&addon)
	return err
}

func (w *watcher) addon(name string) (v1.Addon, error) {
	addon, err := w.addonCache.Get(ns, name)
	if errors.IsNotFound(err) {
		addon = v1.NewAddon(ns, name, v1.Addon{})
	} else if err != nil {
		return v1.Addon{}, err
	}
	return *addon, nil
}

func (w *watcher) apply(addon v1.Addon, set *objectset.ObjectSet) (map[schema.GroupVersionKind]*objectclient.ObjectClient, error) {
	var (
		err error
	)

	op := objectset.NewProcessor(addon.Name)
	op.AllowDiscovery(w.discovery, w.restConfig)

	ds := op.NewDesiredSet(nil, set)

	for _, gvk := range addon.Status.GVKs {
		var (
			namespaced bool
		)

		client, ok := w.clients[gvk]
		if ok {
			namespaced = w.namespaced[gvk]
		} else {
			client, namespaced, err = objectset.NewDiscoveredClient(gvk, w.restConfig, w.discovery)
			if err != nil {
				return nil, err
			}
			if w.namespaced == nil {
				w.namespaced = map[schema.GroupVersionKind]bool{}
			}
			w.namespaced[gvk] = namespaced
		}

		ds.AddDiscoveredClient(gvk, client, namespaced)
	}

	if err := ds.Apply(); err != nil {
		return nil, err
	}

	return ds.DiscoveredClients(), nil
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
