package deploy

import (
	"os"
	"path/filepath"
	"testing"
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
