package etcd

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

func Test_UnitSnapshotRestrictions(t *testing.T) {
	tests := []struct {
		name         string
		restrictions []string
		dataDir      string
		serverDir    string
		serverS3     *config.EtcdS3
		reqDir       *string
		reqS3        *config.EtcdS3

		wantWarnings []string
		wantDir      *string
		// wantS3 is the S3 config expected on the request afterwards. Nil means the request
		// carries no S3 config: either none was sent, or it was dropped.
		wantS3 *config.EtcdS3
	}{
		{
			name:   "no restrictions leaves the request untouched",
			reqDir: stringPtr("/tmp/custom"),
			reqS3: &config.EtcdS3{
				Endpoint: "custom-endpoint",
				Bucket:   "custom-bucket",
				Folder:   "custom-folder",
				Proxy:    "custom-proxy",
			},
			wantDir: stringPtr("/tmp/custom"),
			wantS3: &config.EtcdS3{
				Endpoint: "custom-endpoint",
				Bucket:   "custom-bucket",
				Folder:   "custom-folder",
				Proxy:    "custom-proxy",
			},
		},
		{
			name:         "one restricted field is pinned to the server value, others pass through",
			restrictions: []string{"s3-bucket"},
			serverS3:     &config.EtcdS3{Bucket: "server-bucket", Folder: "server-folder"},
			reqDir:       stringPtr("/tmp/custom"),
			reqS3:        &config.EtcdS3{Bucket: "custom-bucket", Folder: "custom-folder"},
			wantWarnings: []string{"s3-bucket override ignored"},
			wantDir:      stringPtr("/tmp/custom"),
			wantS3:       &config.EtcdS3{Bucket: "server-bucket", Folder: "custom-folder"},
		},
		{
			name:         "restricted target field on a server without S3 drops the S3 section",
			restrictions: []string{"s3-bucket"},
			reqS3:        &config.EtcdS3{Bucket: "custom-bucket", Folder: "custom-folder"},
			wantWarnings: []string{"s3-bucket override ignored"},
			wantS3:       nil,
		},
		{
			name:         "restricted non-target field on a server without S3 keeps the S3 section",
			restrictions: []string{"s3-folder"},
			reqS3:        &config.EtcdS3{Endpoint: "e", Bucket: "b", Folder: "f", Proxy: "p"},
			wantWarnings: []string{"s3-folder override ignored"},
			wantS3:       &config.EtcdS3{Endpoint: "e", Bucket: "b", Folder: "", Proxy: "p"},
		},
		{
			name:         "values equal to the server values emit no warning",
			restrictions: []string{"s3-endpoint", "s3-bucket"},
			serverS3:     &config.EtcdS3{Endpoint: "e", Bucket: "b"},
			reqS3:        &config.EtcdS3{Endpoint: "e", Bucket: "b", Folder: "f"},
			wantS3:       &config.EtcdS3{Endpoint: "e", Bucket: "b", Folder: "f"},
		},
		{
			name:         "multiple restricted fields are listed in order",
			restrictions: []string{"s3-bucket", "s3-endpoint"},
			serverS3:     &config.EtcdS3{Endpoint: "server-endpoint", Bucket: "server-bucket"},
			reqS3:        &config.EtcdS3{Endpoint: "custom-endpoint", Bucket: "custom-bucket", Folder: "custom-folder"},
			wantWarnings: []string{"restricted snapshot options were ignored: s3-endpoint, s3-bucket"},
			wantS3:       &config.EtcdS3{Endpoint: "server-endpoint", Bucket: "server-bucket", Folder: "custom-folder"},
		},
		{
			name:         "snapshot-dir matching the server dir emits no warning",
			restrictions: []string{"snapshot-dir"},
			serverDir:    "/var/lib/server/snapshots",
			reqDir:       stringPtr("/var/lib/server/snapshots"),
		},
		{
			name:         "snapshot-dir is compared after cleaning",
			restrictions: []string{"snapshot-dir"},
			serverDir:    "/var/lib/server/snapshots",
			reqDir:       stringPtr("/var/lib/server/./snapshots/"),
		},
		{
			name:         "snapshot-dir matching the default dir emits no warning",
			restrictions: []string{"snapshot-dir"},
			dataDir:      "/var/lib/rancher/k3s/server",
			reqDir:       stringPtr("/var/lib/rancher/k3s/server/db/snapshots"),
		},
		{
			name:         "snapshot-dir differing from the default dir is ignored",
			restrictions: []string{"snapshot-dir"},
			dataDir:      "/var/lib/rancher/k3s/server",
			reqDir:       stringPtr("/tmp/custom"),
			wantWarnings: []string{"snapshot-dir override ignored"},
		},
		{
			name:         "snapshot-dir is ignored when the server dir is unknown",
			restrictions: []string{"snapshot-dir"},
			reqDir:       stringPtr("/tmp/custom"),
			wantWarnings: []string{"snapshot-dir override ignored"},
		},
		{
			name:         "all with a server S3 config pins every setting and lists them",
			restrictions: []string{"all"},
			dataDir:      "/var/lib/rancher/k3s/server",
			serverS3: &config.EtcdS3{
				Endpoint: "server-endpoint",
				Bucket:   "server-bucket",
				Folder:   "server-folder",
				Proxy:    "server-proxy",
			},
			reqDir: stringPtr("/tmp/custom"),
			reqS3: &config.EtcdS3{
				Endpoint: "custom-endpoint",
				Bucket:   "custom-bucket",
				Folder:   "custom-folder",
				Proxy:    "custom-proxy",
			},
			wantWarnings: []string{"restricted snapshot options were ignored: snapshot-dir, s3-endpoint, s3-bucket, s3-folder, s3-proxy"},
			wantS3: &config.EtcdS3{
				Endpoint: "server-endpoint",
				Bucket:   "server-bucket",
				Folder:   "server-folder",
				Proxy:    "server-proxy",
			},
		},
		{
			name:         "all without a server S3 config drops the S3 section and lists what was ignored",
			restrictions: []string{"all"},
			dataDir:      "/var/lib/rancher/k3s/server",
			reqDir:       stringPtr("/tmp/custom"),
			reqS3: &config.EtcdS3{
				Endpoint: "custom-endpoint",
				Bucket:   "custom-bucket",
				Folder:   "custom-folder",
				Proxy:    "custom-proxy",
			},
			wantWarnings: []string{"restricted snapshot options were ignored: snapshot-dir, s3-endpoint, s3-bucket, s3-folder, s3-proxy"},
			wantS3:       nil,
		},
		{
			name:         "all only reports the overrides that were actually supplied",
			restrictions: []string{"all"},
			dataDir:      "/var/lib/rancher/k3s/server",
			reqDir:       stringPtr("/tmp/custom"),
			wantWarnings: []string{"snapshot-dir override ignored"},
		},
		{
			name:         "restrictions do not apply to a request without an S3 section",
			restrictions: []string{"s3-bucket", "s3-endpoint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ETCD{
				config: &config.Control{
					EtcdSnapshotRestrictions: tt.restrictions,
					DataDir:                  tt.dataDir,
					EtcdSnapshotDir:          tt.serverDir,
					EtcdS3:                   tt.serverS3,
				},
			}
			sr := &SnapshotRequest{Dir: tt.reqDir, S3: tt.reqS3}

			warnings := e.applySnapshotRestrictions(sr)

			if len(warnings) != len(tt.wantWarnings) {
				t.Fatalf("expected %d warnings, got %d: %v", len(tt.wantWarnings), len(warnings), warnings)
			}
			for i, want := range tt.wantWarnings {
				if !strings.Contains(warnings[i], want) {
					t.Errorf("expected warning to contain %q, got %q", want, warnings[i])
				}
			}

			// A restricted request that does not name the server's destination must be
			// cleared; one that is allowed through must be left as it was.
			if !reflect.DeepEqual(sr.Dir, tt.wantDir) {
				t.Errorf("expected Dir %s, got %s", derefString(tt.wantDir), derefString(sr.Dir))
			}
			if !reflect.DeepEqual(sr.S3, tt.wantS3) {
				t.Errorf("expected S3 %+v, got %+v", tt.wantS3, sr.S3)
			}
		})
	}
}

func Test_UnitSnapshotRestrictionsNilSafe(t *testing.T) {
	sr := &SnapshotRequest{Dir: stringPtr("/tmp/custom"), S3: &config.EtcdS3{Bucket: "custom-bucket"}}

	for name, e := range map[string]*ETCD{
		"nil ETCD":   nil,
		"nil config": {},
	} {
		if warnings := e.applySnapshotRestrictions(sr); warnings != nil {
			t.Errorf("%s: expected no warnings, got %v", name, warnings)
		}
	}
	if sr.Dir == nil || sr.S3 == nil || sr.S3.Bucket != "custom-bucket" {
		t.Errorf("request must not be modified when there is nothing to enforce, got Dir=%s S3=%+v", derefString(sr.Dir), sr.S3)
	}
	if warnings := (&ETCD{config: &config.Control{EtcdSnapshotRestrictions: []string{"all"}}}).applySnapshotRestrictions(nil); warnings != nil {
		t.Errorf("nil request: expected no warnings, got %v", warnings)
	}
}

func Test_UnitSnapshotWarningHeaders(t *testing.T) {
	want := []string{"first warning", `second "quoted" warning, with a comma`}

	h := http.Header{}
	addWarnings(h, want)

	got, errs := utilnet.ParseWarningHeaders(h["Warning"])
	if len(errs) != 0 {
		t.Fatalf("failed to parse warning headers %v: %v", h["Warning"], errs)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d warnings, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i].Code != 299 || got[i].Text != want[i] {
			t.Errorf("expected warning 299 %q, got %d %q", want[i], got[i].Code, got[i].Text)
		}
	}

	// A message that cannot be encoded is skipped, without discarding the others.
	h = http.Header{}
	addWarnings(h, []string{"bad\nmessage", "good message"})
	if got, _ := utilnet.ParseWarningHeaders(h["Warning"]); len(got) != 1 || got[0].Text != "good message" {
		t.Errorf("expected only the encodable warning to be sent, got %v", got)
	}
}

func stringPtr(s string) *string {
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func Test_UnitSnapshotWithRequestS3(t *testing.T) {
	e := &ETCD{
		config: &config.Control{
			EtcdS3: &config.EtcdS3{
				Bucket: "server-bucket",
				Folder: "cluster-backups",
				Proxy:  "http://server-proxy:8080",
			},
		},
	}

	// Case 1: User explicitly unsets folder and proxy (sets them to empty string)
	sr := &SnapshotRequest{
		S3: &config.EtcdS3{
			Bucket: "server-bucket",
			Folder: "",
			Proxy:  "",
		},
	}
	re := e.withRequest(sr)
	if re.config.EtcdS3.Folder != "" {
		t.Errorf("expected Folder to be cleared to empty string, got: %s", re.config.EtcdS3.Folder)
	}
	if re.config.EtcdS3.Proxy != "" {
		t.Errorf("expected Proxy to be cleared to empty string, got: %s", re.config.EtcdS3.Proxy)
	}

	// Case 2: sr.S3 is nil -> EtcdS3 should be nil (local snapshot, no S3 upload)
	srNil := &SnapshotRequest{}
	reNil := e.withRequest(srNil)
	if reNil.config.EtcdS3 != nil {
		t.Errorf("expected EtcdS3 to be nil when sr.S3 is nil, got: %v", reNil.config.EtcdS3)
	}
}

func Test_UnitSnapshotHandleDeleteTraversal(t *testing.T) {
	e := &ETCD{config: &config.Control{}}

	tests := []struct {
		name      string
		snapshots []string
		wantErr   bool
	}{
		{
			name:      "relative path traversal with dots",
			snapshots: []string{"../../etc/passwd"},
			wantErr:   true,
		},
		{
			name:      "nested relative traversal",
			snapshots: []string{"foo/../../bar"},
			wantErr:   true,
		},
		{
			name:      "absolute path",
			snapshots: []string{"/etc/passwd"},
			wantErr:   true,
		},
		{
			name:      "valid bare snapshot name",
			snapshots: []string{"on-demand-1234"},
			wantErr:   false,
		},
		{
			name:      "valid name containing consecutive dots",
			snapshots: []string{"on-demand..1234"},
			wantErr:   false,
		},
		{
			name:      "current directory dot",
			snapshots: []string{"."},
			wantErr:   true,
		},
		{
			name:      "parent directory",
			snapshots: []string{".."},
			wantErr:   true,
		},
		{
			name:      "empty snapshot name",
			snapshots: []string{""},
			wantErr:   true,
		},
		{
			name:      "subdirectory name",
			snapshots: []string{"sub/snap"},
			wantErr:   true,
		},
		{
			name:      "trailing separator",
			snapshots: []string{"snap/"},
			wantErr:   true,
		},
		{
			name:      "one invalid name among valid ones",
			snapshots: []string{"on-demand-1234", "../on-demand-5678"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/db/snapshot", nil)
			_ = e.handleDelete(rw, req, tt.snapshots)

			if tt.wantErr {
				if rw.Code != http.StatusBadRequest {
					t.Errorf("expected status %d, got %d", http.StatusBadRequest, rw.Code)
				}
				if !strings.Contains(rw.Body.String(), "path traversal not allowed") {
					t.Errorf("expected error message to contain 'path traversal not allowed', got: %s", rw.Body.String())
				}
			} else {
				if rw.Code == http.StatusBadRequest && strings.Contains(rw.Body.String(), "path traversal not allowed") {
					t.Errorf("did not expect path traversal error for valid snapshot, got: %s", rw.Body.String())
				}
			}
		})
	}
}
