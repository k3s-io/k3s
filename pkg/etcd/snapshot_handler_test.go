package etcd

import (
	"strings"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
)

func Test_UnitSnapshotRestrictions(t *testing.T) {
	tests := []struct {
		name                 string
		restrictions         []string
		reqDir               *string
		reqEndpoint          string
		reqBucket            string
		reqFolder            string
		reqProxy             string
		expectedWarnings     []string
		expectedDir          *string
		expectedEndpoint     string
		expectedBucket       string
		expectedFolder       string
		expectedProxy        string
	}{
		{
			name:                 "no restrictions, overrides applied",
			restrictions:         []string{},
			reqDir:               stringPtr("/tmp/custom"),
			reqEndpoint:          "custom-endpoint",
			reqBucket:            "custom-bucket",
			reqFolder:            "custom-folder",
			reqProxy:             "custom-proxy",
			expectedWarnings:     nil,
			expectedDir:          stringPtr("/tmp/custom"),
			expectedEndpoint:     "custom-endpoint",
			expectedBucket:       "custom-bucket",
			expectedFolder:       "custom-folder",
			expectedProxy:        "custom-proxy",
		},
		{
			name:                 "single restriction bucket",
			restrictions:         []string{"s3-bucket"},
			reqDir:               stringPtr("/tmp/custom"),
			reqBucket:            "custom-bucket",
			reqFolder:            "custom-folder",
			expectedWarnings:     []string{"s3-bucket override ignored"},
			expectedDir:          stringPtr("/tmp/custom"),
			expectedBucket:       "",
			expectedFolder:       "custom-folder",
		},
		{
			name:                 "all restrictions",
			restrictions:         []string{"all"},
			reqDir:               stringPtr("/tmp/custom"),
			reqEndpoint:          "custom-endpoint",
			reqBucket:            "custom-bucket",
			reqFolder:            "custom-folder",
			reqProxy:             "custom-proxy",
			expectedWarnings:     []string{"restricted snapshot options were ignored: all supported destination overrides"},
			expectedDir:          nil,
			expectedEndpoint:     "",
			expectedBucket:       "",
			expectedFolder:       "",
			expectedProxy:        "",
		},
		{
			name:                 "multiple restrictions",
			restrictions:         []string{"s3-endpoint", "s3-bucket"},
			reqEndpoint:          "custom-endpoint",
			reqBucket:            "custom-bucket",
			reqFolder:            "custom-folder",
			expectedWarnings:     []string{"restricted snapshot options were ignored: s3-endpoint, s3-bucket"},
			expectedEndpoint:     "",
			expectedBucket:       "",
			expectedFolder:       "custom-folder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &ETCD{
				config: &config.Control{
					EtcdSnapshotRestrictions: tt.restrictions,
				},
			}
			sr := &SnapshotRequest{
				Dir: tt.reqDir,
				S3: &config.EtcdS3{
					Endpoint: tt.reqEndpoint,
					Bucket:   tt.reqBucket,
					Folder:   tt.reqFolder,
					Proxy:    tt.reqProxy,
				},
			}

			warnings := e.applySnapshotRestrictions(sr)

			// Check warnings
			if len(tt.expectedWarnings) == 0 && len(warnings) != 0 {
				t.Fatalf("expected no warnings, got: %v", warnings)
			}
			for i, expW := range tt.expectedWarnings {
				if i >= len(warnings) {
					t.Fatalf("missing expected warning: %s", expW)
				}
				if !strings.Contains(warnings[i], expW) {
					t.Errorf("expected warning to contain '%s', got: '%s'", expW, warnings[i])
				}
			}

			// Check mutated request
			if tt.expectedDir != nil {
				if sr.Dir == nil || *sr.Dir != *tt.expectedDir {
					t.Errorf("expected Dir %v, got %v", *tt.expectedDir, sr.Dir)
				}
			} else if sr.Dir != nil {
				t.Errorf("expected Dir nil, got %v", *sr.Dir)
			}

			if sr.S3.Endpoint != tt.expectedEndpoint {
				t.Errorf("expected Endpoint %s, got %s", tt.expectedEndpoint, sr.S3.Endpoint)
			}
			if sr.S3.Bucket != tt.expectedBucket {
				t.Errorf("expected Bucket %s, got %s", tt.expectedBucket, sr.S3.Bucket)
			}
			if sr.S3.Folder != tt.expectedFolder {
				t.Errorf("expected Folder %s, got %s", tt.expectedFolder, sr.S3.Folder)
			}
			if sr.S3.Proxy != tt.expectedProxy {
				t.Errorf("expected Proxy %s, got %s", tt.expectedProxy, sr.S3.Proxy)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
