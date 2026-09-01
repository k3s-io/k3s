package deploy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	apisv1 "github.com/k3s-io/api/k3s.cattle.io/v1"
	clientsetfake "github.com/k3s-io/api/pkg/generated/clientset/versioned/fake"
	applyfake "github.com/rancher/wrangler/v3/pkg/apply/fake"
	genericfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

func Test_UnitWalkFilesSymlinkedDirectoryUsesLogicalPaths(t *testing.T) {
	base := t.TempDir()
	target := t.TempDir()

	manifest := filepath.Join(target, "nested", "manifest.yaml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(base, "linked")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	files, err := walkFiles(base)
	if err != nil {
		t.Fatal(err)
	}

	logicalPath := filepath.Join(link, "nested", "manifest.yaml")
	entry, ok := files[logicalPath]
	if !ok {
		t.Fatalf("expected symlinked manifest to use logical path %q; got %v", logicalPath, fileKeys(files))
	}
	if _, ok := files[manifest]; ok {
		t.Fatalf("expected resolved target path %q to be hidden by logical symlink path", manifest)
	}
	if entry.removeOnDisable {
		t.Fatalf("expected symlinked manifest %q to avoid on-disk removal when disabled", logicalPath)
	}
	if !shouldDisableFile(base, logicalPath, map[string]bool{"linked": true}) {
		t.Fatalf("expected logical path %q to match disabled symlink directory", logicalPath)
	}
	if !shouldDisableFile(base, logicalPath, map[string]bool{"manifest": true}) {
		t.Fatalf("expected logical path %q to match disabled basename", logicalPath)
	}
}

func Test_UnitWalkFilesRegularManifestRemainsRemovable(t *testing.T) {
	base := t.TempDir()
	manifest := filepath.Join(base, "regular.yaml")
	if err := os.WriteFile(manifest, []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: regular\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := walkFiles(base)
	if err != nil {
		t.Fatal(err)
	}
	if !files[manifest].removeOnDisable {
		t.Fatalf("expected regular manifest %q to remain removable when disabled", manifest)
	}
}

func fileKeys(files map[string]watchedFile) []string {
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	return keys
}

// newTestWatcher returns a watcher whose Addon cache and client calls are wired
// to a fake clientset. The fake clientset doesn't assign UIDs like the real API
// server does, so the Addon is pre-created with one set; deploy() treats an
// empty UID as a new Addon and attempts to create it.
func newTestWatcher(t *testing.T, apply *applyfake.FakeApply) *watcher {
	t.Helper()

	clientset := clientsetfake.NewSimpleClientset(&apisv1.Addon{ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceSystem, Name: "manifest", UID: "test-uid"}})
	addons := clientset.K3sV1().Addons(metav1.NamespaceSystem)

	ctrl := gomock.NewController(t)
	addonCache := genericfake.NewMockCacheInterface[*apisv1.Addon](ctrl)
	addonClient := genericfake.NewMockClientInterface[*apisv1.Addon, *apisv1.AddonList](ctrl)
	addonCache.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(_, name string) (*apisv1.Addon, error) {
		return addons.Get(t.Context(), name, metav1.GetOptions{})
	})
	addonClient.EXPECT().Create(gomock.Any()).AnyTimes().DoAndReturn(func(a *apisv1.Addon) (*apisv1.Addon, error) {
		return addons.Create(t.Context(), a, metav1.CreateOptions{})
	})
	addonClient.EXPECT().Update(gomock.Any()).AnyTimes().DoAndReturn(func(a *apisv1.Addon) (*apisv1.Addon, error) {
		return addons.Update(t.Context(), a, metav1.UpdateOptions{})
	})

	return &watcher{
		apply:      apply,
		addonCache: addonCache,
		addons:     addonClient,
		modTime:    map[string]time.Time{},
		recorder:   record.NewFakeRecorder(100),
		discovery:  clientset.Discovery(),
	}
}

func Test_UnitListFilesInAppliesOnContentChangeDespiteUnchangedModTime(t *testing.T) {
	base := t.TempDir()
	manifest := filepath.Join(base, "manifest.yaml")
	contentA := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n"
	contentB := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n  labels:\n    changed: \"true\"\n"

	// Simulate filesystems (e.g. NixOS /nix/store) that pin all mtimes to the
	// epoch, so mtime never changes even when file content does.
	fixedMtime := time.Unix(0, 0)

	writeManifest := func(content string) {
		t.Helper()
		if err := os.WriteFile(manifest, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(manifest, fixedMtime, fixedMtime); err != nil {
			t.Fatal(err)
		}
	}

	apply := &applyfake.FakeApply{}
	w := newTestWatcher(t, apply)

	// First pass is forced, so the manifest is applied and its checksum recorded.
	writeManifest(contentA)
	if err := w.listFilesIn(base, true); err != nil {
		t.Fatal(err)
	}
	if apply.Count != 1 {
		t.Fatalf("expected initial apply, got %d", apply.Count)
	}

	// Second (non-forced) pass must detect the content change via checksum even
	// though mtime is unchanged, and apply it again.
	writeManifest(contentB)
	if err := w.listFilesIn(base, false); err != nil {
		t.Fatal(err)
	}
	if apply.Count != 2 {
		t.Fatalf("expected changed content to be applied despite unchanged mtime, got %d applies", apply.Count)
	}
}

func Test_UnitListFilesInSkipsUnchangedNormalModTime(t *testing.T) {
	base := t.TempDir()
	manifest := filepath.Join(base, "manifest.yaml")
	contentA := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n"
	contentB := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: test\n  labels:\n    changed: \"true\"\n"

	// A normal (non-epoch) mtime: the modTime gate trusts it and skips the file
	// when it is unchanged.
	fixedMtime := time.Unix(1000000, 0)

	writeManifest := func(content string) {
		t.Helper()
		if err := os.WriteFile(manifest, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(manifest, fixedMtime, fixedMtime); err != nil {
			t.Fatal(err)
		}
	}

	apply := &applyfake.FakeApply{}
	w := newTestWatcher(t, apply)

	// First pass is forced, so the manifest is applied and its checksum recorded.
	writeManifest(contentA)
	if err := w.listFilesIn(base, true); err != nil {
		t.Fatal(err)
	}
	if apply.Count != 1 {
		t.Fatalf("expected initial apply, got %d", apply.Count)
	}

	// Content changes but mtime does not: files with a normal mtime are skipped
	// based on mtime alone, per the gate retained in listFilesIn.
	writeManifest(contentB)
	if err := w.listFilesIn(base, false); err != nil {
		t.Fatal(err)
	}
	if apply.Count != 1 {
		t.Fatalf("expected file with unchanged normal mtime to be skipped, got %d applies", apply.Count)
	}
}
