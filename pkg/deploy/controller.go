package deploy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/util"
	apisv1 "github.com/k3s-io/api/k3s.cattle.io/v1"
	controllersv1 "github.com/k3s-io/api/pkg/generated/controllers/k3s.cattle.io/v1"
	pkgutil "github.com/k3s-io/k3s/pkg/util"
	errors2 "github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/rancher/wrangler/v3/pkg/objectset"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
)

const (
	ControllerName = "deploy"
	GVKAnnotation  = "addon.k3s.cattle.io/gvks"
	startKey       = "_start_"
	gvkSep         = ";"
)

// WatchFiles sets up an OnChange callback to start a periodic goroutine to watch files for changes once the controller has started up.
func WatchFiles(ctx context.Context, client kubernetes.Interface, apply apply.Apply, addons controllersv1.AddonController, disables map[string]bool, bases ...string) error {
	w := &watcher{
		apply:      apply,
		addonCache: addons.Cache(),
		addons:     addons,
		bases:      bases,
		disables:   disables,
		modTime:    map[string]time.Time{},
		gvkCache:   map[schema.GroupVersionKind]bool{},
		discovery:  client.Discovery(),
	}

	addons.Enqueue(metav1.NamespaceNone, startKey)
	addons.OnChange(ctx, "addon-start", func(key string, _ *apisv1.Addon) (*apisv1.Addon, error) {
		if key == startKey {
			go w.start(ctx, client)
		}
		return nil, nil
	})

	return nil
}

type watcher struct {
	sync.Mutex

	apply      apply.Apply
	addonCache controllersv1.AddonCache
	addons     controllersv1.AddonClient
	bases      []string
	disables   map[string]bool
	modTime    map[string]time.Time
	gvkCache   map[schema.GroupVersionKind]bool
	recorder   record.EventRecorder
	discovery  discovery.DiscoveryInterface
}

// start calls listFiles at regular intervals to trigger application of manifests that have changed on disk.
func (w *watcher) start(ctx context.Context, client kubernetes.Interface) {
	w.recorder = pkgutil.BuildControllerEventRecorder(client, ControllerName, metav1.NamespaceSystem)
	force := true
	for {
		if err := w.listFiles(force); err == nil {
			force = false
		} else {
			logrus.Errorf("Failed to process config: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}

// listFiles calls listFilesIn on a list of paths.
func (w *watcher) listFiles(force bool) error {
	var errs []error
	for _, base := range w.bases {
		if err := w.listFilesIn(base, force); err != nil {
			errs = append(errs, err)
		}
	}
	return merr.NewErrors(errs...)
}

// listFilesIn recursively processes all files within a path, and checks them against the disable and skip lists. Files found that
// are not on either list are loaded as Addons and applied to the cluster.
func (w *watcher) listFilesIn(base string, force bool) error {
	files := map[string]os.FileInfo{}
	if err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Descend into symlinked directories, however, only top-level links are followed
		if info.Mode()&os.ModeSymlink != 0 {
			linkInfo, err := os.Stat(path)
			if err != nil {
				return err
			}
			if linkInfo.IsDir() {
				evalPath, err := filepath.EvalSymlinks(path)
				if err != nil {
					return err
				}
				filepath.Walk(evalPath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					files[path] = info
					return nil
				})
			}
		}
		files[path] = info
		return nil
	}); err != nil {
		return err
	}

	// Make a map of .skip files - these are used later to indicate that a given file should be ignored
	// For example, 'addon.yaml.skip' will cause 'addon.yaml' to be ignored completely - unless it is also
	// disabled, since disable processing happens first.
	skips := map[string]bool{}
	keys := make([]string, len(files))
	keyIndex := 0
	for path, file := range files {
		if strings.HasSuffix(file.Name(), ".skip") {
			skips[strings.TrimSuffix(file.Name(), ".skip")] = true
		}
		keys[keyIndex] = path
		keyIndex++
	}
	sort.Strings(keys)

	var errs []error
	for _, path := range keys {
		// Disabled files are not just skipped, but actively deleted from the filesystem
		if shouldDisableFile(base, path, w.disables) {
			if err := w.delete(path); err != nil {
				errs = append(errs, errors2.Wrapf(err, "failed to delete %s", path))
			}
			continue
		}
		// Skipped files are just ignored
		if shouldSkipFile(files[path].Name(), skips) {
			continue
		}
		modTime := files[path].ModTime()
		if !force && modTime.Equal(w.modTime[path]) {
			continue
		}
		if err := w.deploy(path, !force); err != nil {
			errs = append(errs, errors2.Wrapf(err, "failed to process %s", path))
		} else {
			w.modTime[path] = modTime
		}
	}

	return merr.NewErrors(errs...)
}

// deploy loads yaml from a manifest on disk, creates an AddOn resource to track its application, and then applies
// all resources contained within to the cluster.
func (w *watcher) deploy(path string, compareChecksum bool) error {
	name := basename(path)
	addon, err := w.getOrCreateAddon(name)
	if err != nil {
		return err
	}

	addon.Spec.Source = path

	// Create the new Addon now so that we can use it to report Events when parsing/applying the manifest
	// Events need the UID and ObjectRevision set to function properly
	if addon.UID == "" {
		newAddon, err := w.addons.Create(&addon)
		if err != nil {
			return err
		}
		addon = *newAddon
	}

	content, err := os.ReadFile(path)
	if err != nil {
		w.recorder.Eventf(&addon, corev1.EventTypeWarning, "ReadManifestFailed", "Read manifest at %q failed: %v", path, err)
		return err
	}

	checksum := checksum(content)
	if compareChecksum && checksum == addon.Spec.Checksum {
		logrus.Debugf("Skipping existing deployment of %s, check=%v, checksum %s=%s", path, compareChecksum, checksum, addon.Spec.Checksum)
		return nil
	}

	// Attempt to parse the YAML/JSON into objects. Failure at this point would be due to bad file content - not YAML/JSON,
	// YAML/JSON that can't be converted to Kubernetes objects, etc.
	objects, err := objectSet(content)
	if err != nil {
		w.recorder.Eventf(&addon, corev1.EventTypeWarning, "ParseManifestFailed", "Parse manifest at %q failed: %v", path, err)
		return err
	}

	// Merge GVK list early for validation
	addonGVKs := objects.GVKs()
	for _, gvkString := range strings.Split(addon.Annotations[GVKAnnotation], gvkSep) {
		if gvk, err := getGVK(gvkString); err == nil {
			addonGVKs = append(addonGVKs, *gvk)
		}
	}

	// Ensure that we don't try to prune using GVKs that the server doesn't have.
	// This can happen when CRDs are removed or when core types are removed - PodSecurityPolicy, for example.
	addonGVKs, err = w.validateGVKs(addonGVKs)
	if err != nil {
		w.recorder.Eventf(&addon, corev1.EventTypeWarning, "ValidateManifestFailed", "Validate GVKs for manifest at %q failed: %v", path, err)
		return err
	}

	// Attempt to apply the changes. Failure at this point would be due to more complicated issues - invalid changes to
	// existing objects, rejected by validating webhooks, etc.
	// WithGVK searches for objects using both GVKs currently listed in the manifest, as well as GVKs previously
	// applied.  This ensures that objects don't get orphaned when they are removed from the file - if the apply
	// doesn't know to search that GVK for owner references, it won't find and delete them.
	w.recorder.Eventf(&addon, corev1.EventTypeNormal, "ApplyingManifest", "Applying manifest at %q", path)

	if err := w.apply.WithOwner(&addon).WithGVK(addonGVKs...).Apply(objects); err != nil {
		w.recorder.Eventf(&addon, corev1.EventTypeWarning, "ApplyManifestFailed", "Applying manifest at %q failed: %v", path, err)
		return err
	}

	// Emit event, Update Addon checksum and GVKs only if apply was successful
	w.recorder.Eventf(&addon, corev1.EventTypeNormal, "AppliedManifest", "Applied manifest at %q", path)
	if addon.Annotations == nil {
		addon.Annotations = map[string]string{}
	}
	addon.Spec.Checksum = checksum
	addon.Annotations[GVKAnnotation] = getGVKString(objects.GVKs())
	_, err = w.addons.Update(&addon)
	return err
}

// delete completely removes both a manifest, and any resources that it did or would have created. The manifest is
// parsed, and any resources it specified are deleted. Finally, the file itself is removed from disk.
func (w *watcher) delete(path string) error {
	name := basename(path)
	addon, err := w.getOrCreateAddon(name)
	if err != nil {
		return err
	}

	addonGVKs := []schema.GroupVersionKind{}
	for _, gvkString := range strings.Split(addon.Annotations[GVKAnnotation], gvkSep) {
		if gvk, err := getGVK(gvkString); err == nil {
			addonGVKs = append(addonGVKs, *gvk)
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		w.recorder.Eventf(&addon, corev1.EventTypeWarning, "ReadManifestFailed", "Read manifest at %q failed: %v", path, err)
	} else {
		if o, err := objectSet(content); err != nil {
			w.recorder.Eventf(&addon, corev1.EventTypeWarning, "ParseManifestFailed", "Parse manifest at %q failed: %v", path, err)
		} else {
			// Search for objects using both GVKs currently listed in the file, as well as GVKs previously applied.
			// This ensures that any conflicts between competing deploy controllers are handled properly.
			addonGVKs = append(addonGVKs, o.GVKs()...)
		}
	}

	// Ensure that we don't try to delete using GVKs that the server doesn't have.
	// This can happen when CRDs are removed or when core types are removed - PodSecurityPolicy, for example.
	addonGVKs, err = w.validateGVKs(addonGVKs)
	if err != nil {
		return err
	}

	// ensure that the addon is completely removed before deleting the objectSet,
	// so return when err == nil, otherwise pods may get stuck terminating
	w.recorder.Eventf(&addon, corev1.EventTypeNormal, "DeletingManifest", "Deleting manifest at %q", path)
	if err := w.addons.Delete(addon.Namespace, addon.Name, &metav1.DeleteOptions{}); err == nil || !apierrors.IsNotFound(err) {
		return err
	}

	// apply an empty set with owner & gvk data to delete
	if err := w.apply.WithOwner(&addon).WithGVK(addonGVKs...).ApplyObjects(); err != nil {
		return err
	}

	return os.Remove(path)
}

// getOrCreateAddon attempts to get an Addon by name from the addon namespace, and creates a new one
// if it cannot be found.
func (w *watcher) getOrCreateAddon(name string) (apisv1.Addon, error) {
	addon, err := w.addonCache.Get(metav1.NamespaceSystem, name)
	if apierrors.IsNotFound(err) {
		addon = apisv1.NewAddon(metav1.NamespaceSystem, name, apisv1.Addon{})
	} else if err != nil {
		return apisv1.Addon{}, err
	}
	return *addon, nil
}

// validateGVKs removes from the list any GVKs that the server does not support
func (w *watcher) validateGVKs(addonGVKs []schema.GroupVersionKind) ([]schema.GroupVersionKind, error) {
	gvks := []schema.GroupVersionKind{}
	for _, gvk := range addonGVKs {
		found, err := w.serverHasGVK(gvk)
		if err != nil {
			return gvks, err
		}
		if found {
			gvks = append(gvks, gvk)
		}
	}
	return gvks, nil
}

// serverHasGVK uses a positive cache of GVKs that the cluster is known to have supported at some
// point in time.  Note this may fail to filter out GVKs that are removed from the cluster after
// startup (for example, if CRDs are deleted) - but the Wrangler DesiredSet cache has the same issue,
// so it should be fine.
func (w *watcher) serverHasGVK(gvk schema.GroupVersionKind) (bool, error) {
	w.Lock()
	defer w.Unlock()

	if found, ok := w.gvkCache[gvk]; ok {
		return found, nil
	}

	resources, err := w.discovery.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Cache all Kinds for this GroupVersion to save on future lookups
	for _, resource := range resources.APIResources {
		// Resources in the requested GV are returned with empty GroupVersion.
		// Subresources with different GV may also be returned, but we aren't interested in those.
		if resource.Group == "" && resource.Version == "" {
			w.gvkCache[gvk.GroupVersion().WithKind(resource.Kind)] = true
		}
	}

	return w.gvkCache[gvk], nil
}

// objectSet returns a new ObjectSet containing all resources from a given yaml chunk
func objectSet(content []byte) (*objectset.ObjectSet, error) {
	objs, err := yamlToObjects(bytes.NewBuffer(content))
	if err != nil {
		return nil, err
	}

	return objectset.NewObjectSet(objs...), nil
}

// basename returns a file's basename by returning everything before the first period
func basename(path string) string {
	name := filepath.Base(path)
	return strings.SplitN(name, ".", 2)[0]
}

// checksum returns the hex-encoded SHA256 sum of a byte slice
func checksum(bytes []byte) string {
	d := sha256.Sum256(bytes)
	return hex.EncodeToString(d[:])
}

// isEmptyYaml returns true if a chunk of YAML contains nothing but whitespace, comments, or document separators
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

// yamlToObjects returns an object slice yielded from documents in a chunk of YAML
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

// Returns one or more objects from a single YAML document
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

// Returns true if a file should be skipped. Skips anything from the provided skip map,
// anything that is a dotfile, and anything that does not have a json/yaml/yml extension.
func shouldSkipFile(fileName string, skips map[string]bool) bool {
	switch {
	case strings.HasPrefix(fileName, "."):
		return true
	case skips[fileName]:
		return true
	case util.HasSuffixI(fileName, ".yaml", ".yml", ".json"):
		return false
	default:
		return true
	}
}

// Returns true if a file should be disabled, by checking the file basename against a disables map.
// only json/yaml files are checked.
func shouldDisableFile(base, fileName string, disables map[string]bool) bool {
	// Check to see if the file is in a subdirectory that is in the disables map.
	// If a file is nested several levels deep, checks 'parent1', 'parent1/parent2', 'parent1/parent2/parent3', etc.
	relFile := strings.TrimPrefix(fileName, base)
	namePath := strings.Split(relFile, string(os.PathSeparator))
	for i := 1; i < len(namePath); i++ {
		subPath := filepath.Join(namePath[0:i]...)
		if disables[subPath] {
			return true
		}
	}
	if !util.HasSuffixI(fileName, ".yaml", ".yml", ".json") {
		return false
	}
	// Check the basename against the disables map
	baseFile := filepath.Base(fileName)
	suffix := filepath.Ext(baseFile)
	baseName := strings.TrimSuffix(baseFile, suffix)
	return disables[baseName]
}

func getGVK(s string) (*schema.GroupVersionKind, error) {
	parts := strings.Split(s, ", Kind=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid GVK format: %s", s)
	}
	gvk := &schema.GroupVersionKind{}
	gvk.Group, gvk.Version = kv.Split(parts[0], "/")
	gvk.Kind = parts[1]
	return gvk, nil
}

func getGVKString(gvks []schema.GroupVersionKind) string {
	strs := make([]string, len(gvks))
	for i, gvk := range gvks {
		strs[i] = gvk.String()
	}
	return strings.Join(strs, gvkSep)
}
