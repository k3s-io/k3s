package etcd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd/snapshot"
)

func Test_UnitETCD_restorePath(t *testing.T) {
	const content = "not really an etcd snapshot"

	// writeSnapshot writes a fake snapshot file into dir and returns its path.
	writeSnapshot := func(t *testing.T, dir string) string {
		t.Helper()
		path := filepath.Join(dir, "snapshot-to-restore")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("failed to write snapshot: %v", err)
		}
		return path
	}

	t.Run("uncompressed snapshot is used as-is", func(t *testing.T) {
		path := writeSnapshot(t, t.TempDir())
		e := &ETCD{config: &config.Control{ClusterResetRestorePath: path}}

		got, err := e.restorePath()
		if err != nil {
			t.Fatalf("ETCD.restorePath() error = %v", err)
		}
		if got != path {
			t.Errorf("ETCD.restorePath() = %q, want %q", got, path)
		}
	})

	t.Run("compressed snapshot is decompressed alongside the archive", func(t *testing.T) {
		dir := t.TempDir()
		path := writeSnapshot(t, dir)
		e := &ETCD{config: &config.Control{}}

		zipPath, err := e.compressSnapshot(dir, filepath.Base(path), time.Now())
		if err != nil {
			t.Fatalf("failed to compress snapshot: %v", err)
		}
		// drop the plaintext, so that the assertions below can only be
		// satisfied by the contents of the archive
		if err := os.Remove(path); err != nil {
			t.Fatalf("failed to remove the uncompressed snapshot: %v", err)
		}
		e.config.ClusterResetRestorePath = zipPath

		got, err := e.restorePath()
		if err != nil {
			t.Fatalf("ETCD.restorePath() error = %v", err)
		}
		if got != path {
			t.Errorf("ETCD.restorePath() = %q, want %q", got, path)
		}
		if b, err := os.ReadFile(got); err != nil || string(b) != content {
			t.Errorf("restored snapshot = %q (err = %v), want %q", b, err, content)
		}
	})

	t.Run("missing compressed snapshot is an error", func(t *testing.T) {
		e := &ETCD{config: &config.Control{
			ClusterResetRestorePath: filepath.Join(t.TempDir(), "missing"+snapshot.CompressedExtension),
		}}

		if got, err := e.restorePath(); err == nil {
			t.Errorf("ETCD.restorePath() = %q, want an error", got)
		}
	})
}
