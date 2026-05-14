package etcd

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/etcd/snapshot"
)

func Test_UnitETCD_compressSnapshot(t *testing.T) {
	const (
		snapshotName = "on-demand-test-node-1700000000"
		payload      = "fake etcd snapshot contents"
	)

	dir := t.TempDir()
	src := filepath.Join(dir, snapshotName)
	if err := os.WriteFile(src, []byte(payload), 0600); err != nil {
		t.Fatalf("failed to seed source snapshot: %v", err)
	}

	e := &ETCD{}
	zipPath, err := e.compressSnapshot(dir, snapshotName, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("compressSnapshot returned error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(zipPath) })

	wantPath := src + snapshot.CompressedExtension
	if zipPath != wantPath {
		t.Errorf("zipPath = %q, want %q", zipPath, wantPath)
	}

	info, err := os.Stat(zipPath)
	if err != nil {
		t.Fatalf("stat compressed snapshot: %v", err)
	}

	// The compressed snapshot can contain the same etcd data as the
	// uncompressed file, so its on-disk mode must be just as restrictive.
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("compressed snapshot mode = %#o, want 0600", got)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("opening zip: %v", err)
	}
	t.Cleanup(func() { _ = zr.Close() })

	if len(zr.File) != 1 {
		t.Fatalf("zip contains %d files, want 1", len(zr.File))
	}
	if zr.File[0].Name != snapshotName {
		t.Errorf("zip entry name = %q, want %q", zr.File[0].Name, snapshotName)
	}

	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("opening zip entry: %v", err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		t.Fatalf("reading zip entry: %v", err)
	}
	if buf.String() != payload {
		t.Errorf("zip entry contents = %q, want %q", buf.String(), payload)
	}
}
