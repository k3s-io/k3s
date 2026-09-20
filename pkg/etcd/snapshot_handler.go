package etcd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	k3s "github.com/k3s-io/api/k3s.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"fmt"
	"strings"

	"path/filepath"
)

type SnapshotOperation string

const (
	SnapshotOperationSave   SnapshotOperation = "save"
	SnapshotOperationList   SnapshotOperation = "list"
	SnapshotOperationPrune  SnapshotOperation = "prune"
	SnapshotOperationDelete SnapshotOperation = "delete"
)

type SnapshotRequest struct {
	Operation SnapshotOperation `json:"operation"`
	Name      []string          `json:"name,omitempty"`
	Dir       *string           `json:"dir,omitempty"`
	Compress  *bool             `json:"compress,omitempty"`
	Retention *int              `json:"retention,omitempty"`
	S3        *config.EtcdS3    `json:"s3,omitempty"`

	ctx context.Context
}

func (sr *SnapshotRequest) context() context.Context {
	if sr.ctx != nil {
		return sr.ctx
	}
	return context.Background()
}

// snapshotHandler handles snapshot save/list/prune requests from the CLI.
func (e *ETCD) snapshotHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		sr, err := getSnapshotRequest(req)
		if err != nil {
			util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
			return
		}

		warnings := e.applySnapshotRestrictions(sr)
		for _, w := range warnings {
			rw.Header().Add("Warning", fmt.Sprintf("299 - \"%s\"", w))
		}

		switch sr.Operation {
		case SnapshotOperationList:
			err = e.withRequest(sr).handleList(rw, req)
		case SnapshotOperationSave:
			err = e.withRequest(sr).handleSave(rw, req)
		case SnapshotOperationPrune:
			err = e.withRequest(sr).handlePrune(rw, req)
		case SnapshotOperationDelete:
			err = e.withRequest(sr).handleDelete(rw, req, sr.Name)
		default:
			err = e.handleInvalid(rw, req)
		}
		if err != nil {
			logrus.Warnf("Error in etcd-snapshot handler: %v", err)
		}
	})
}

func (e *ETCD) handleList(rw http.ResponseWriter, req *http.Request) error {
	if e.config != nil && e.config.EtcdS3 != nil {
		if _, err := e.getS3Client(req.Context()); err != nil {
			err = errors.WithMessage(err, "failed to initialize S3 client")
			util.SendError(err, rw, req, http.StatusBadRequest)
			return nil
		}
	}
	sf, err := e.ListSnapshots(req.Context())
	if sf == nil {
		util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		return nil
	}
	sendSnapshotList(rw, req, sf)
	return err
}

func (e *ETCD) handleSave(rw http.ResponseWriter, req *http.Request) error {
	if e.config.EtcdS3 != nil {
		if _, err := e.getS3Client(req.Context()); err != nil {
			err = errors.WithMessage(err, "failed to initialize S3 client")
			util.SendError(err, rw, req, http.StatusBadRequest)
			return nil
		}
	}
	sr, err := e.Snapshot(req.Context())
	if sr == nil {
		util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		return nil
	}
	sendSnapshotResponse(rw, req, sr)
	return err
}

func (e *ETCD) handlePrune(rw http.ResponseWriter, req *http.Request) error {
	if e.config.EtcdS3 != nil {
		if _, err := e.getS3Client(req.Context()); err != nil {
			err = errors.WithMessage(err, "failed to initialize S3 client")
			util.SendError(err, rw, req, http.StatusBadRequest)
			return nil
		}
	}
	sr, err := e.PruneSnapshots(req.Context())
	if sr == nil {
		util.SendError(err, rw, req, http.StatusInternalServerError)
		return nil
	}
	sendSnapshotResponse(rw, req, sr)
	return err
}

func (e *ETCD) handleDelete(rw http.ResponseWriter, req *http.Request, snapshots []string) error {
	for _, snapshot := range snapshots {
		cleaned := filepath.Clean(snapshot)
		if cleaned == "." || cleaned == "" || strings.Contains(cleaned, "..") || filepath.IsAbs(cleaned) || filepath.Base(cleaned) != cleaned {
			util.SendError(errors.New("invalid snapshot name: path traversal not allowed"), rw, req, http.StatusBadRequest)
			return nil
		}
	}
	if e.config.EtcdS3 != nil {
		if _, err := e.getS3Client(req.Context()); err != nil {
			err = errors.WithMessage(err, "failed to initialize S3 client")
			util.SendError(err, rw, req, http.StatusBadRequest)
			return nil
		}
	}
	sr, err := e.DeleteSnapshots(req.Context(), snapshots)
	if sr == nil {
		util.SendError(err, rw, req, http.StatusInternalServerError)
		return nil
	}
	sendSnapshotResponse(rw, req, sr)
	return err
}

func (e *ETCD) handleInvalid(rw http.ResponseWriter, req *http.Request) error {
	util.SendErrorWithID(errors.New("invalid snapshot operation"), "etcd-snapshot", rw, req, http.StatusBadRequest)
	return nil
}

// withRequest returns a modified ETCD struct that is overridden
// with etcd snapshot config from the snapshot request.
func (e *ETCD) withRequest(sr *SnapshotRequest) *ETCD {
	re := &ETCD{
		client: e.client,
		config: &config.Control{
			CriticalControlArgs:   e.config.CriticalControlArgs,
			Runtime:               e.config.Runtime,
			DataDir:               e.config.DataDir,
			Datastore:             e.config.Datastore,
			DisableAgent:          e.config.DisableAgent,
			EtcdSnapshotCompress:  e.config.EtcdSnapshotCompress,
			EtcdSnapshotName:      e.config.EtcdSnapshotName,
			EtcdSnapshotDir:       e.config.EtcdSnapshotDir,
			EtcdSnapshotRetention: e.config.EtcdSnapshotRetention,
			EtcdS3:                sr.S3,
		},
		s3:         e.s3,
		name:       e.name,
		address:    e.address,
		cron:       e.cron,
		snapshotMu: e.snapshotMu,
	}
	if len(sr.Name) > 0 {
		re.config.EtcdSnapshotName = sr.Name[0]
	}
	if sr.Compress != nil {
		re.config.EtcdSnapshotCompress = *sr.Compress
	}
	if sr.Dir != nil {
		re.config.EtcdSnapshotDir = *sr.Dir
	}
	if sr.Retention != nil {
		re.config.EtcdSnapshotRetention = *sr.Retention
	}
	return re
}

// getSnapshotRequest unmarshalls the snapshot operation request from a client.
func getSnapshotRequest(req *http.Request) (*SnapshotRequest, error) {
	if req.Method != http.MethodPost {
		return nil, apierrors.NewMethodNotSupported(k3s.Resource("snapshot"), req.Method)
	}
	sr := &SnapshotRequest{}
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &sr); err != nil {
		return nil, err
	}
	sr.ctx = req.Context()
	return sr, nil
}

func sendSnapshotResponse(rw http.ResponseWriter, req *http.Request, sr *managed.SnapshotResult) {
	b, err := json.Marshal(sr)
	if err != nil {
		util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(b)
}

func sendSnapshotList(rw http.ResponseWriter, req *http.Request, sf *k3s.ETCDSnapshotFileList) {
	b, err := json.Marshal(sf)
	if err != nil {
		util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(b)
}

func (e *ETCD) applySnapshotRestrictions(sr *SnapshotRequest) []string {
	if len(e.config.EtcdSnapshotRestrictions) == 0 {
		return nil
	}

	restrictions := sets.New(e.config.EtcdSnapshotRestrictions...)
	hasAll := restrictions.Has("all")
	isRestricted := func(field string) bool {
		return hasAll || restrictions.Has(field)
	}

	var ignored []string

	if isRestricted("snapshot-dir") && sr.Dir != nil {
		if e.config.EtcdSnapshotDir == "" || *sr.Dir != e.config.EtcdSnapshotDir {
			ignored = append(ignored, "snapshot-dir")
		}
		sr.Dir = nil
	}

	if sr.S3 != nil {
		serverS3 := e.config.EtcdS3
		getExpected := func(getter func(*config.EtcdS3) string) string {
			if serverS3 != nil {
				return getter(serverS3)
			}
			return ""
		}

		if isRestricted("s3-endpoint") && sr.S3.Endpoint != getExpected(func(s *config.EtcdS3) string { return s.Endpoint }) {
			ignored = append(ignored, "s3-endpoint")
			sr.S3.Endpoint = getExpected(func(s *config.EtcdS3) string { return s.Endpoint })
		}
		if isRestricted("s3-bucket") && sr.S3.Bucket != getExpected(func(s *config.EtcdS3) string { return s.Bucket }) {
			ignored = append(ignored, "s3-bucket")
			sr.S3.Bucket = getExpected(func(s *config.EtcdS3) string { return s.Bucket })
		}
		if isRestricted("s3-folder") && sr.S3.Folder != getExpected(func(s *config.EtcdS3) string { return s.Folder }) {
			ignored = append(ignored, "s3-folder")
			sr.S3.Folder = getExpected(func(s *config.EtcdS3) string { return s.Folder })
		}
		if isRestricted("s3-proxy") && sr.S3.Proxy != getExpected(func(s *config.EtcdS3) string { return s.Proxy }) {
			ignored = append(ignored, "s3-proxy")
			sr.S3.Proxy = getExpected(func(s *config.EtcdS3) string { return s.Proxy })
		}
	}

	if len(ignored) == 0 {
		return nil
	}

	if hasAll && len(ignored) > 1 {
		return []string{"restricted snapshot options were ignored: all supported destination overrides. Using the server-configured snapshot destination."}
	} else if len(ignored) > 1 {
		return []string{fmt.Sprintf("restricted snapshot options were ignored: %s. Using the server-configured snapshot destination.", strings.Join(ignored, ", "))}
	}
	
	return []string{fmt.Sprintf("%s override ignored by server-side snapshot restrictions. Using the server-configured destination.", ignored[0])}
}
