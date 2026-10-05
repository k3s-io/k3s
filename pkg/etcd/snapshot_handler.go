package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	k3s "github.com/k3s-io/api/k3s.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
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
	ctx       context.Context
}

func (s *SnapshotRequest) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
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

		addWarnings(rw.Header(), e.applySnapshotRestrictions(sr))

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
	for _, name := range snapshots {
		if !validSnapshotName(name) {
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

// snapshotRestrictionAll is the --etcd-snapshot-restrictions value that restricts every
// supported setting.
const snapshotRestrictionAll = "all"

// restrictedS3Field describes an S3 setting that --etcd-snapshot-restrictions can pin to the
// server-configured value.
type restrictedS3Field struct {
	// name is the value accepted by --etcd-snapshot-restrictions.
	name string
	// value returns a pointer to the setting within an S3 config.
	value func(*config.EtcdS3) *string
	// identifiesTarget is true if the setting selects which S3 service or bucket snapshots are
	// stored in, as opposed to refining how or where within it (folder, proxy).
	identifiesTarget bool
}

// restrictedS3Fields lists the S3 settings that can be restricted, in the order they are
// reported to the client.
var restrictedS3Fields = []restrictedS3Field{
	{name: "s3-endpoint", value: func(s *config.EtcdS3) *string { return &s.Endpoint }, identifiesTarget: true},
	{name: "s3-bucket", value: func(s *config.EtcdS3) *string { return &s.Bucket }, identifiesTarget: true},
	{name: "s3-folder", value: func(s *config.EtcdS3) *string { return &s.Folder }},
	{name: "s3-proxy", value: func(s *config.EtcdS3) *string { return &s.Proxy }},
}

// effectiveSnapshotDir returns the directory the server stores snapshots in: the configured
// snapshot directory, or the default one. It returns an empty string if neither is known.
func effectiveSnapshotDir(c *config.Control) string {
	if c.EtcdSnapshotDir != "" {
		return c.EtcdSnapshotDir
	}
	if c.DataDir == "" {
		return ""
	}
	return defaultSnapshotPath(c)
}

// applySnapshotRestrictions enforces the server's --etcd-snapshot-restrictions on a snapshot
// request. Any restricted setting that the request tries to override with something other than
// the server-configured value is reset to the server-configured value, and a warning naming
// what was ignored is returned for the client. The request is never rejected, and is modified
// in place.
//
// The server-configured value of an S3 setting is empty if the server has no S3 configuration.
// If the request tries to override a setting that identifies the S3 target (endpoint or bucket)
// in that case, there is no server-configured target to fall back to, so the S3 section is
// dropped and the request is handled using the server-configured (local) destination instead.
func (e *ETCD) applySnapshotRestrictions(sr *SnapshotRequest) []string {
	if e == nil || e.config == nil || sr == nil || len(e.config.EtcdSnapshotRestrictions) == 0 {
		return nil
	}

	restrictions := sets.New(e.config.EtcdSnapshotRestrictions...)
	all := restrictions.Has(snapshotRestrictionAll)
	isRestricted := func(name string) bool {
		return all || restrictions.Has(name)
	}

	var ignored []string

	if sr.Dir != nil && isRestricted("snapshot-dir") {
		serverDir := effectiveSnapshotDir(e.config)
		if serverDir == "" || filepath.Clean(*sr.Dir) != filepath.Clean(serverDir) {
			ignored = append(ignored, "snapshot-dir")
		}
		sr.Dir = nil
	}

	if sr.S3 != nil {
		serverS3 := e.config.EtcdS3
		dropS3 := false
		for _, field := range restrictedS3Fields {
			if !isRestricted(field.name) {
				continue
			}
			pinned := ""
			if serverS3 != nil {
				pinned = *field.value(serverS3)
			}
			requested := field.value(sr.S3)
			if *requested == pinned {
				continue
			}
			ignored = append(ignored, field.name)
			*requested = pinned
			if field.identifiesTarget && serverS3 == nil {
				dropS3 = true
			}
		}
		if dropS3 {
			sr.S3 = nil
		}
	}

	switch len(ignored) {
	case 0:
		return nil
	case 1:
		return []string{fmt.Sprintf("%s override ignored by server-side snapshot restrictions. Using the server-configured destination.", ignored[0])}
	default:
		return []string{fmt.Sprintf("restricted snapshot options were ignored: %s. Using the server-configured snapshot destination.", strings.Join(ignored, ", "))}
	}
}

// addWarnings sends each message to the client as an RFC 7234 "299" Warning header, which the
// CLI prints when it reads the response.
func addWarnings(h http.Header, warnings []string) {
	for _, w := range warnings {
		value, err := utilnet.NewWarningHeader(299, "", w)
		if err != nil {
			logrus.Warnf("Unable to send warning to etcd-snapshot client: %v: %s", err, w)
			continue
		}
		h.Add("Warning", value)
	}
}

// validSnapshotName returns true if name is a plain snapshot file name. Names are joined onto
// the snapshot directory and used as S3 object keys, so anything that is a path - one with
// separators, an absolute path, or "." / ".." - could address something other than a snapshot.
func validSnapshotName(name string) bool {
	return name != "" && name != "." && name != ".." && filepath.Base(name) == name
}
