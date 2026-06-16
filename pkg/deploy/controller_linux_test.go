//go:build linux

package deploy

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Test_UnitScanManifests covers the path-rewriting behavior that makes
// --disable=<symlinkName> match files under a top-level symlinked manifests
// directory. Each entry's map key is the file's logical path under base; the
// realPath records the on-disk location, which differs only for entries
// reached through a top-level symlink.
func Test_UnitScanManifests(t *testing.T) {
	base := t.TempDir()
	external := t.TempDir()

	// Plain file directly under base.
	directPath := filepath.Join(base, "direct.yaml")
	if err := os.WriteFile(directPath, []byte("---\n"), 0600); err != nil {
		t.Fatalf("write direct: %v", err)
	}

	// Regular subdirectory under base with a file inside.
	subDir := filepath.Join(base, "sub")
	if err := os.Mkdir(subDir, 0700); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	subPath := filepath.Join(subDir, "child.yaml")
	if err := os.WriteFile(subPath, []byte("---\n"), 0600); err != nil {
		t.Fatalf("write sub child: %v", err)
	}

	// Top-level symlink under base pointing at an external directory with a
	// nested manifest inside.
	extNestedDir := filepath.Join(external, "nested")
	if err := os.Mkdir(extNestedDir, 0700); err != nil {
		t.Fatalf("mkdir external/nested: %v", err)
	}
	extFile := filepath.Join(extNestedDir, "test.yaml")
	if err := os.WriteFile(extFile, []byte("---\n"), 0600); err != nil {
		t.Fatalf("write external file: %v", err)
	}
	linkPath := filepath.Join(base, "linked")
	if err := os.Symlink(external, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	files, err := scanManifests(base)
	if err != nil {
		t.Fatalf("scanManifests: %v", err)
	}

	want := map[string]string{
		base:                              base,
		directPath:                        directPath,
		subDir:                            subDir,
		subPath:                           subPath,
		linkPath:                          linkPath, // the symlink itself stays local
		filepath.Join(linkPath, "nested"): extNestedDir,
		filepath.Join(linkPath, "nested", "test.yaml"): extFile,
	}

	if len(files) != len(want) {
		gotKeys := make([]string, 0, len(files))
		for k := range files {
			gotKeys = append(gotKeys, k)
		}
		sort.Strings(gotKeys)
		t.Fatalf("scanManifests returned %d entries, want %d. got: %v", len(files), len(want), gotKeys)
	}
	for logical, wantReal := range want {
		entry, ok := files[logical]
		if !ok {
			t.Errorf("missing entry for logical path %q", logical)
			continue
		}
		if entry.realPath != wantReal {
			t.Errorf("entry %q: realPath = %q, want %q", logical, entry.realPath, wantReal)
		}
	}
}

// Test_UnitShouldDisableFile_Symlinked covers the disable-matching fix: with
// the logical paths produced by scanManifests, --disable=<symlinkName> must
// match files under that symlinked directory.
func Test_UnitShouldDisableFile_Symlinked(t *testing.T) {
	base := "/var/lib/rancher/k3s/server/manifests/"
	disables := map[string]bool{"linked": true}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "file under symlinked dir", path: base + "linked/test.yaml", want: true},
		{name: "nested file under symlinked dir", path: base + "linked/nested/test.yaml", want: true},
		{name: "unrelated file", path: base + "other.yaml", want: false},
		{name: "file in non-disabled subdir", path: base + "other/test.yaml", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldDisableFile(base, tt.path, disables); got != tt.want {
				t.Errorf("shouldDisableFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
